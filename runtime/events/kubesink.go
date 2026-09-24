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

// KubeBackend selects which Kubernetes Event API the Recorder writes to. Pass a
// value to WithManager to override the default. The zero value is the core/v1
// backend.
type KubeBackend int

const (
	// backendCoreV1 writes Events through the legacy core/v1 recorder. It is
	// the default and preserves the full event message. It is unexported
	// because it is the zero value and never needs to be named explicitly.
	backendCoreV1 KubeBackend = iota

	// EventsV1 writes Events through the events.k8s.io/v1 recorder. It records
	// the related object and action on the Event, but the apiserver rejects
	// notes longer than 1024 bytes (see kubeSink). Use it only when event
	// messages are known to stay within that limit.
	EventsV1
)

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
// The seam is internal: the note-length asymmetry means the choice must stay
// under this package's control rather than becoming part of the public API.
type kubeSink interface {
	// emit records a Kubernetes Event for the given object.
	//
	// related and action are accepted for parity across backends. Backends
	// whose Event representation has no field for them (core/v1) drop them;
	// they are always preserved on the Flux event/v1 webhook payload
	// regardless of the backend.
	emit(object, related runtime.Object, annotations map[string]string,
		eventType, reason, action, messageFmt string, args ...interface{})
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

// eventsV1Sink wraps the events.k8s.io/v1 recorder
// (k8s.io/client-go/tools/events). It preserves the related object and action
// on the Kubernetes Event.
//
// WARNING: this backend always sets EventTime, so the apiserver rejects any
// note longer than 1024 bytes (pkg/apis/core/validation.NoteLengthLimit) and
// the event is silently dropped without an error surfaced to the caller. Flux
// events routinely exceed that limit (applied resource lists, dry-run diffs,
// Helm upgrade errors). Use it only when messages are known to stay within the
// limit. It is never the default; select it with WithManager(mgr, EventsV1).
type eventsV1Sink struct {
	recorder events.AnnotatedEventRecorder
}

// eventsV1Sink implements kubeSink.
var _ kubeSink = &eventsV1Sink{}

func (s *eventsV1Sink) emit(object, related runtime.Object, annotations map[string]string,
	eventType, reason, action, messageFmt string, args ...interface{}) {
	s.recorder.AnnotatedEventf(object, related, annotations, eventType, reason, action, messageFmt, args...)
}
