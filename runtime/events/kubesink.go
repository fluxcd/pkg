/*
Copyright 2026 The Flux authors

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	kuberecorder "k8s.io/client-go/tools/record"
)

// EventsAPI selects the Kubernetes Events API the Recorder writes to. Pass a
// value to WithEventsAPI to override the package default. The empty value
// means the package default.
type EventsAPI string

const (
	// EventsAPICoreV1 writes Events through the legacy core/v1 recorder
	// (k8s.io/client-go/tools/record).
	EventsAPICoreV1 EventsAPI = "v1"

	// EventsAPIEventsV1 writes Events through the events.k8s.io/v1 recorder
	// (k8s.io/client-go/tools/events).
	EventsAPIEventsV1 EventsAPI = "events.k8s.io/v1"
)

// defaultEventsAPI is the Kubernetes Events API used when the caller does not
// select one explicitly. It is the single line to flip to change the default.
const defaultEventsAPI = EventsAPICoreV1

// kubeSink abstracts the Kubernetes Event backend used by the Recorder.
//
// It exists so the recorder can write Kubernetes Events through either the
// legacy core/v1 recorder or the newer events.k8s.io/v1 recorder without
// changing the controller-facing Recorder interface. The two backends are not
// interchangeable:
//
//   - core/v1 (k8s.io/client-go/tools/record) does not set EventTime and so is
//     not subject to the apiserver's strict note-length validation. Flux events
//     keep their full message. This is the default.
//   - events.k8s.io/v1 (k8s.io/client-go/tools/events) always sets EventTime,
//     which triggers strict validation and rejects any note longer than 1024
//     bytes (pkg/apis/core/validation.NoteLengthLimit). It carries a dedicated
//     related-object and action on the Event, but silently drops events whose
//     message exceeds the limit. It is available only as an explicit opt-in.
//
// kubeSink is the internal seam that keeps this asymmetry out of the
// controller-facing Recorder interface. Callers select a backend with the
// public EventsAPI values via WithEventsAPI (or implicitly through
// WithEventRecorder / WithLegacyEventRecorder); the concrete sink stays
// unexported.
type kubeSink interface {
	// emit records a Kubernetes Event for the given object.
	//
	// related and action are accepted for parity across backends. Backends
	// whose Event representation has no field for them (core/v1) drop them;
	// they are always preserved on the Flux event/v1 webhook payload
	// regardless of the backend.
	emit(object, related runtime.Object, annotations map[string]string,
		eventType, reason, action, messageFmt string, args ...interface{})

	// api reports which Kubernetes Events API this sink writes to. It lets
	// NewRecorder detect a conflict between an injected recorder and an
	// explicit WithEventsAPI selection.
	api() EventsAPI
}

// coreV1Sink is the default kubeSink. It wraps the legacy core/v1 event
// recorder (k8s.io/client-go/tools/record).
//
// The core/v1 Event type has no dedicated related-object or action field, so
// both are dropped here; they are preserved on the Flux event/v1 webhook
// payload. Because the core/v1 recorder does not set EventTime, notes are not
// subject to the apiserver's 1024-byte limit and the full message survives on
// the Event.
type coreV1Sink struct {
	recorder kuberecorder.EventRecorder
}

// coreV1Sink implements kubeSink.
var _ kubeSink = &coreV1Sink{}

func (s *coreV1Sink) emit(object, _ runtime.Object, annotations map[string]string,
	eventType, reason, _, messageFmt string, args ...interface{}) {
	s.recorder.AnnotatedEventf(object, annotations, eventType, reason, messageFmt, args...)
}

func (s *coreV1Sink) api() EventsAPI { return EventsAPICoreV1 }

// eventsV1Sink wraps the events.k8s.io/v1 recorder
// (k8s.io/client-go/tools/events). It preserves the related object and action
// on the Kubernetes Event.
//
// WARNING: this backend always sets EventTime, so the apiserver rejects any
// note longer than 1024 bytes (pkg/apis/core/validation.NoteLengthLimit) and
// the event is silently dropped without an error surfaced to the caller. Flux
// events routinely exceed that limit (applied resource lists, dry-run diffs,
// Helm upgrade errors). Use it only when messages are known to stay within the
// limit. It is never the default; select it with WithEventsAPI(EventsAPIEventsV1).
type eventsV1Sink struct {
	recorder events.AnnotatedEventRecorder
}

// eventsV1Sink implements kubeSink.
var _ kubeSink = &eventsV1Sink{}

func (s *eventsV1Sink) emit(object, related runtime.Object, annotations map[string]string,
	eventType, reason, action, messageFmt string, args ...interface{}) {
	s.recorder.AnnotatedEventf(object, related, annotations, eventType, reason, action, messageFmt, args...)
}

func (s *eventsV1Sink) api() EventsAPI { return EventsAPIEventsV1 }
