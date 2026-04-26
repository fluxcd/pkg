/*
Copyright 2020 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-retryablehttp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	"github.com/fluxcd/pkg/runtime/logger"
)

// Recorder posts events to the Kubernetes API and any other event recorder webhook address, like the GitOps Toolkit
// notification-controller.
//
// Use it by embedding EventRecorder in reconciler struct:
//
//	import (
//		...
//		"k8s.io/client-go/tools/events"
//		...
//	)
//
//	type MyTypeReconciler {
//	 	client.Client
//		// ... etc.
//		events.EventRecorder
//	}
//
// Use NewRecorder to create a working Recorder.
type EventRecorder interface {
	// Eventf records an event with a given/formatted message.
	Eventf(object runtime.Object, related runtime.Object, eventtype, reason, action, messageFmt string, args ...interface{})

	// AnnotatedEventf records an event with annotations and a formatted message.
	AnnotatedEventf(object runtime.Object, related runtime.Object, annotations map[string]string, eventtype, reason, action, messageFmt string, args ...interface{})
}

type recorder struct {
	// URL address of the events endpoint.
	webhook string

	// Name of the controller that emits events.
	reportingController string

	// Retryable HTTP client.
	client *retryablehttp.Client

	// EventRecorder is the Kubernetes event recorder.
	events.EventRecorder

	// Scheme to look up the recorded objects.
	scheme *runtime.Scheme

	// Log is the recorder logger.
	log logr.Logger
}

var _ events.EventRecorder = &recorder{}

// RecorderOption configures a recorder.
type RecorderOption func(*recorder)

// NewRecorder creates an events.EventRecorder with a Kubernetes event recorder
// and an external event recorder based on the given webhook. The recorder
// performs automatic retries for connection errors and 500-range response codes
// from the external recorder.
func NewRecorder(mgr ctrl.Manager, log logr.Logger, webhook, reportingController string, opts ...RecorderOption) (EventRecorder, error) {
	return NewRecorderForScheme(mgr.GetScheme(), mgr.GetEventRecorder(reportingController), log, webhook, reportingController, opts...)
}

// NewRecorderForScheme creates an events.EventRecorder with a Kubernetes event
// recorder and an external event recorder based on the given webhook. The
// recorder performs automatic retries for connection errors and 500-range
// response codes from the external recorder.
func NewRecorderForScheme(scheme *runtime.Scheme,
	eventRecorder events.EventRecorder,
	log logr.Logger, webhook, reportingController string, opts ...RecorderOption) (EventRecorder, error) {
	if webhook != "" {
		if _, err := url.Parse(webhook); err != nil {
			return nil, err
		}
	}

	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient.Timeout = 5 * time.Second
	httpClient.CheckRetry = checkRetry
	httpClient.Logger = nil

	r := &recorder{
		scheme:              scheme,
		webhook:             webhook,
		reportingController: reportingController,
		client:              httpClient,
		EventRecorder:       eventRecorder,
		log:                 log,
	}
	for _, o := range opts {
		o(r)
	}
	return r, nil
}

func WithRetryMax(n int) RecorderOption {
	return func(r *recorder) {
		r.client.RetryMax = n
	}
}

func checkRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if resp != nil && responseIsEventDuplicated(resp) {
		return false, nil // Don't retry
	}
	return retryablehttp.ErrorPropagatedRetryPolicy(ctx, resp, err)
}

// responseIsEventDuplicated checks if the received response is a signal of a duplicate event.
// The Notification Controller returns a 429 Too-Many-Requests response when the posted message
// is a duplicate (within a certain time window).
func responseIsEventDuplicated(resp *http.Response) bool {
	return resp.StatusCode == http.StatusTooManyRequests
}

// Event records an event in the webhook address.
func (r *recorder) Eventf(object runtime.Object, related runtime.Object, eventtype, reason string, action string, messageFmt string, args ...interface{}) {
	r.AnnotatedEventf(object, related, nil, eventtype, reason, action, messageFmt, args...)
}

// AnnotatedEventf constructs an event from the given information and performs a HTTP POST to the webhook address.
// It also logs the event if debug logs are enabled in the logger.
func (r *recorder) AnnotatedEventf(
	object runtime.Object,
	related runtime.Object,
	inputAnnotations map[string]string,
	eventtype, reason string,
	action string,
	messageFmt string, args ...interface{}) {

	ref, err := reference.GetReference(r.scheme, object)
	if err != nil {
		r.log.Error(err, "failed to get object reference")
	}

	// Add object annotations to the annotations.
	annotations := maps.Clone(inputAnnotations)
	if annotatedObject, ok := object.(interface{ GetAnnotations() map[string]string }); ok {
		for k, v := range annotatedObject.GetAnnotations() {
			if strings.HasPrefix(k, eventv1.Group+"/") {
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations[k] = v
			}
		}
	}

	// Add object info in the logger.
	log := r.log.WithValues("name", ref.Name, "namespace", ref.Namespace, "reconciler kind", ref.Kind)

	// Log the event if in trace mode.
	if log.GetSink().Enabled(logger.TraceLevel) {
		msg := fmt.Sprintf(messageFmt, args...)
		if eventtype == corev1.EventTypeWarning {
			log.Error(errors.New(reason), msg, "annotations", annotations)
		} else {
			log.Info(msg, "reason", reason, "annotations", annotations)
		}
	}

	// Convert the eventType to severity.
	severity := eventTypeToSeverity(eventtype)

	// Do not send trace events to notification controller,
	// traces are persisted as Kubernetes events only as normal events.
	if severity == eventv1.EventSeverityTrace {
		r.EventRecorder.Eventf(object, related, corev1.EventTypeNormal, reason, action, messageFmt, args...)
		return
	}

	// Forward the event to the Kubernetes recorder.
	r.EventRecorder.Eventf(object, related, eventtype, reason, action, messageFmt, args...)

	// If no webhook address is provided, skip posting to event recorder
	// endpoint.
	if r.webhook == "" {
		return
	}

	if r.client == nil {
		err := fmt.Errorf("retryable HTTP client has not been initialized")
		log.Error(err, "unable to record event")
		return
	}

	message := fmt.Sprintf(messageFmt, args...)

	if ref.Kind == "" {
		err := fmt.Errorf("failed to get object kind")
		log.Error(err, "unable to record event")
		return
	}

	if ref.Name == "" {
		err := fmt.Errorf("failed to get object name")
		log.Error(err, "unable to record event")
		return
	}

	if ref.Namespace == "" {
		err := fmt.Errorf("failed to get object namespace")
		log.Error(err, "unable to record event")
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Error(err, "failed to get hostname")
		return
	}

	event := eventv1.Event{
		InvolvedObject:      *ref,
		Severity:            severity,
		Timestamp:           metav1.Now(),
		Message:             message,
		Reason:              reason,
		Action:              action,
		Metadata:            annotations,
		ReportingController: r.reportingController,
		ReportingInstance:   hostname,
	}

	// Add related object reference if provided (optional).
	relatedRef, err := reference.GetReference(r.scheme, related)
	if err == nil {
		event.RelatedObject = *relatedRef
	}

	body, err := json.Marshal(event)
	if err != nil {
		log.Error(err, "failed to marshal object into json")
		return
	}

	if _, err := r.client.Post(r.webhook, "application/json", body); err != nil {
		log.Error(err, "unable to record event")
		return
	}
}

// eventTypeToSeverity maps the given eventType string to a GOTK event severity
// type.
func eventTypeToSeverity(eventType string) string {
	switch eventType {
	case corev1.EventTypeWarning:
		return eventv1.EventSeverityError
	case eventv1.EventTypeTrace:
		return eventv1.EventSeverityTrace
	default:
		return eventv1.EventSeverityInfo
	}
}
