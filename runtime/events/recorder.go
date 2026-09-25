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
	kuberecorder "k8s.io/client-go/tools/record"
	"k8s.io/client-go/tools/reference"
	ctrl "sigs.k8s.io/controller-runtime"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1"
	"github.com/fluxcd/pkg/runtime/logger"
)

// Recorder posts events to the Kubernetes API and any other event recorder webhook address, like the GitOps Toolkit
// notification-controller.
//
// Use it by embedding Recorder in reconciler struct:
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
//		events.Recorder
//	}
//
// Use NewRecorder to create a working Recorder.
type Recorder interface {
	// EventRecorder records events with a formatted message.
	events.EventRecorder

	// AnnotatedEventRecorder records events with annotations and a formatted message.
	events.AnnotatedEventRecorder
}

type recorder struct {
	// URL address of the events endpoint.
	webhook string

	// Name of the controller that emits events.
	reportingController string

	// Retryable HTTP client.
	client *retryablehttp.Client

	// kube is the Kubernetes Event backend.
	//
	// It defaults to a coreV1Sink, which wraps the legacy core/v1 recorder
	// (k8s.io/client-go/tools/record).
	//
	// The events.k8s.io/v1 backend (eventsV1Sink) is available as an explicit
	// opt-in via WithManager(mgr, EventsV1). See kubeSink for the trade-offs.
	kube kubeSink

	// Scheme to look up the recorded objects.
	scheme *runtime.Scheme

	// Log is the recorder logger.
	log logr.Logger
}

var _ Recorder = &recorder{}

// RecorderOption configures a recorder.
type RecorderOption func(*recorder)

// NewRecorder creates a Recorder with a Kubernetes event recorder and an
// external event recorder based on the given webhook. The recorder performs
// automatic retries for connection errors and 500-range response codes from the
// external recorder.
//
// The scheme and Kubernetes event recorder can be provided either via
// WithManager (common case) or via WithScheme and WithEventRecorderFor
// (for tests or custom setups).
//
// By default the Kubernetes Event backend is the legacy core/v1 recorder,
// which preserves the full event message. To use the events.k8s.io/v1 backend
// instead, pass EventsV1 to WithManager, noting the 1024-byte note limit
// documented there.
func NewRecorder(log logr.Logger, webhook, reportingController string, opts ...RecorderOption) (Recorder, error) {
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
		webhook:             webhook,
		reportingController: reportingController,
		client:              httpClient,
		log:                 log,
	}
	for _, o := range opts {
		o(r)
	}
	return r, nil
}

// WithManager configures the recorder with the scheme and Kubernetes Event
// backend from the given controller-runtime manager. The Kubernetes event
// recorder reports under the reportingController passed to NewRecorder.
//
// By default it uses the legacy core/v1 recorder, which preserves the full
// event message. Pass EventsV1 to use the events.k8s.io/v1 backend instead:
//
//	events.WithManager(mgr)                  // core/v1 (default)
//	events.WithManager(mgr, events.EventsV1) // events.k8s.io/v1
//
// The events.k8s.io/v1 backend records the related object and action on the
// Event, but the apiserver rejects notes longer than 1024 bytes
// (pkg/apis/core/validation.NoteLengthLimit) and silently drops the event
// without surfacing an error. Use it only when event messages are known to
// stay within the limit.
func WithManager(mgr ctrl.Manager, backend ...KubeBackend) RecorderOption {
	return func(r *recorder) {
		r.scheme = mgr.GetScheme()
		b := backendCoreV1
		if len(backend) > 0 {
			b = backend[len(backend)-1]
		}
		if b == EventsV1 {
			r.kube = &eventsV1Sink{recorder: mgr.GetEventRecorder(r.reportingController)}
			return
		}
		// Use the legacy core/v1 event recorder to avoid the events.k8s.io
		// 1024-byte note limit. See the kube field documentation.
		r.kube = &coreV1Sink{recorder: mgr.GetEventRecorderFor(r.reportingController)}
	}
}

// WithScheme configures the recorder with the given runtime scheme for
// resolving object references. Use this together with WithEventRecorderFor
// for tests or custom setups where a ctrl.Manager is not available.
func WithScheme(scheme *runtime.Scheme) RecorderOption {
	return func(r *recorder) {
		r.scheme = scheme
	}
}

// WithEventRecorderFor configures the recorder with the given legacy core/v1
// Kubernetes event recorder. Use this together with WithScheme for tests or
// custom setups where a ctrl.Manager is not available.
//
// The recorder is a core/v1 recorder (k8s.io/client-go/tools/record) so that
// recorded notes are not subject to the events.k8s.io 1024-byte limit. See the
// kube field documentation.
func WithEventRecorderFor(er kuberecorder.EventRecorder) RecorderOption {
	return func(r *recorder) {
		r.kube = &coreV1Sink{recorder: er}
	}
}

// WithRetryMax configures the maximum number of retries for the HTTP client.
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
		return
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

	// Emit the Kubernetes Event via the configured backend. The default
	// core/v1 backend has no related-object or action field and drops them;
	// the events.k8s.io/v1 backend records them. Both are always preserved on
	// the Flux event/v1 payload sent to the webhook below.
	//
	// A recorder configured without a Kubernetes Event backend (e.g. only
	// WithScheme for a webhook-only setup) skips this sink rather than
	// panicking; the webhook payload below is still sent.
	if r.kube != nil {
		// Do not send trace events to notification controller,
		// traces are persisted as Kubernetes events only as normal events.
		if severity == eventv1.EventSeverityTrace {
			r.kube.emit(object, related, annotations, corev1.EventTypeNormal, reason, action, messageFmt, args...)
			return
		}

		// Forward the event to the Kubernetes recorder.
		r.kube.emit(object, related, annotations, eventtype, reason, action, messageFmt, args...)
	} else if severity == eventv1.EventSeverityTrace {
		// Trace events are only ever persisted as Kubernetes Events; with no
		// Kubernetes backend there is nothing to do and they are not webhooked.
		return
	}

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
		event.RelatedObject = relatedRef
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
