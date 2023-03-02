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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-retryablehttp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
	"github.com/fluxcd/pkg/runtime/logger"
)

// Recorder posts events to the Kubernetes API and any other event recorder webhook address, like the GitOps Toolkit
// notification-controller.
//
// Use it by embedding EventRecorder in reconciler struct:
//
//	import (
//		...
//		kuberecorder "k8s.io/client-go/tools/record"
//		...
//	)
//
//	type MyTypeReconciler {
//	 	client.Client
//		// ... etc.
//		kuberecorder.EventRecorder
//	}
//
// Use NewRecorder to create a working Recorder.
type Recorder struct {
	// URL address of the events endpoint.
	Webhook string

	// Name of the controller that emits events.
	ReportingController string

	// Retryable HTTP client.
	Client *retryablehttp.Client

	// EventRecorder is the Kubernetes event recorder.
	EventRecorder kuberecorder.EventRecorder

	// Scheme to look up the recorded objects.
	Scheme *runtime.Scheme

	// Log is the recorder logger.
	Log logr.Logger
}

var _ kuberecorder.EventRecorder = &Recorder{}

// NewRecorder creates an event Recorder with a Kubernetes event recorder and an external event recorder based on the
// given webhook. The recorder performs automatic retries for connection errors and 500-range response codes from the
// external recorder.
func NewRecorder(mgr ctrl.Manager, log logr.Logger, webhook, reportingController string) (*Recorder, error) {
	if webhook != "" {
		if _, err := url.Parse(webhook); err != nil {
			return nil, err
		}
	}

	httpClient := retryablehttp.NewClient()
	httpClient.HTTPClient.Timeout = 5 * time.Second
	httpClient.CheckRetry = retryablehttp.ErrorPropagatedRetryPolicy
	httpClient.Logger = nil

	return &Recorder{
		Scheme:              mgr.GetScheme(),
		Webhook:             webhook,
		ReportingController: reportingController,
		Client:              httpClient,
		EventRecorder:       mgr.GetEventRecorderFor(reportingController),
		Log:                 log,
	}, nil
}

// Event records an event in the webhook address.
func (r *Recorder) Event(object runtime.Object, eventtype, reason, message string) {
	r.AnnotatedEventf(object, nil, eventtype, reason, message)
}

// Event records an event in the webhook address.
func (r *Recorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
	r.AnnotatedEventf(object, nil, eventtype, reason, messageFmt, args...)
}

// AnnotatedEventf constructs an event from the given information and performs a HTTP POST to the webhook address.
// It also logs the event if debug logs are enabled in the logger.
func (r *Recorder) AnnotatedEventf(
	object runtime.Object,
	annotations map[string]string,
	eventtype, reason string,
	messageFmt string, args ...interface{}) {

	ref, err := reference.GetReference(r.Scheme, object)
	if err != nil {
		r.Log.Error(err, "failed to get object reference")
	}

	// Add object info in the logger.
	log := r.Log.WithValues("name", ref.Name, "namespace", ref.Namespace, "reconciler kind", ref.Kind)

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
		r.EventRecorder.AnnotatedEventf(object, annotations, corev1.EventTypeNormal, reason, messageFmt, args...)
		return
	}

	// Forward the event to the Kubernetes recorder.
	r.EventRecorder.AnnotatedEventf(object, annotations, eventtype, reason, messageFmt, args...)

	// If no webhook address is provided, skip posting to event recorder
	// endpoint.
	if r.Webhook == "" {
		return
	}

	if r.Client == nil {
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
		Metadata:            annotations,
		ReportingController: r.ReportingController,
		ReportingInstance:   hostname,
	}

	body, err := json.Marshal(event)
	if err != nil {
		log.Error(err, "failed to marshal object into json")
		return
	}

	// avoid retrying rate limited requests
	if res, _ := r.Client.HTTPClient.Post(r.Webhook, "application/json", bytes.NewReader(body)); res != nil &&
		(res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusAccepted) {
		return
	}

	if _, err := r.Client.Post(r.Webhook, "application/json", body); err != nil {
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
