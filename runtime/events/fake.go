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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	_ "k8s.io/client-go/tools/events"
)

// FakeRecorder is used as a fake during tests.
//
// It was invented to be used in tests which require more precise control over
// e.g. assertions of specific event fields like Reason. For which string
// comparisons on the concentrated event message using record.FakeRecorder is
// not sufficient.
//
// To empty the Events channel into a slice of the recorded events, use
// GetEvents(). Not initializing Events will cause the recorder to not record
// any messages.
type FakeRecorder struct {
	Events        chan eventsv1.Event
	IncludeObject bool
}

// NewFakeRecorder creates new fake event recorder with an Events channel with
// the given size. Setting includeObject to true will cause the recorder to
// include the object reference in the events.
//
// To initialize a recorder which does not record any events, simply use:
//
//	recorder := new(FakeRecorder)
func NewFakeRecorder(bufferSize int, includeObject bool) *FakeRecorder {
	return &FakeRecorder{
		Events:        make(chan eventsv1.Event, bufferSize),
		IncludeObject: includeObject,
	}
}

// NewNopRecorder creates a new FakeRecorder that doesn't record any events.
// This is the most lightweight option for tests that don't need event recording
// functionality. The recorder will implement the EventRecorder interface but
// all event calls will be no-ops.
//
// Example:
//
//	r := &MyReconciler{
//		Client:        k8sClient,
//		EventRecorder: events.NewNopRecorder(),
//	}
func NewNopRecorder() *FakeRecorder {
	return &FakeRecorder{
		Events:        nil,
		IncludeObject: false,
	}
}

// Event emits an event with the given message.
func (f *FakeRecorder) Event(obj runtime.Object, related runtime.Object, eventType, reason, action, message string) {
	f.AnnotatedEventf(obj, related, nil, eventType, reason, action, "%s", message)
}

// Eventf emits an event with the given message.
func (f *FakeRecorder) Eventf(obj runtime.Object, related runtime.Object, eventType, reason, action, message string, args ...interface{}) {
	if f.Events != nil {
		f.Events <- f.generateEvent(obj, related, nil, eventType, reason, action, message, args...)
	}
}

// AnnotatedEventf emits an event with annotations.
func (f *FakeRecorder) AnnotatedEventf(obj runtime.Object, related runtime.Object,
	annotations map[string]string,
	eventType, reason, action,
	message string, args ...interface{}) {
	if f.Events != nil {
		f.Events <- f.generateEvent(obj, related, annotations, eventType, reason, action, message, args...)
	}
}

// GetEvents empties the Events channel and returns a slice of recorded events.
// If the Events channel is nil, it returns nil.
func (f *FakeRecorder) GetEvents() (events []eventsv1.Event) {
	if f.Events != nil {
		for {
			select {
			case e := <-f.Events:
				events = append(events, e)
			default:
				return events
			}
		}
	}
	return nil
}

// generateEvent generates a new mocked event with the given parameters.
func (f *FakeRecorder) generateEvent(obj runtime.Object, related runtime.Object,
	annotations map[string]string,
	eventType, reason, action,
	message string, args ...interface{}) eventsv1.Event {
	event := eventsv1.Event{
		Regarding: objectReference(obj, f.IncludeObject),
		Type:      eventType,
		Reason:    reason,
		Action:    action,
		Note:      fmt.Sprintf(message, args...),
	}
	if annotations != nil {
		event.ObjectMeta.Annotations = annotations
	}

	if related != nil {
		relatedRef := objectReference(related, f.IncludeObject)
		event.Related = &relatedRef
	}

	return event
}

// objectReference returns an object reference for the given object with the
// kind and (group) API version set.
func objectReference(obj runtime.Object, includeObject bool) corev1.ObjectReference {
	if !includeObject {
		return corev1.ObjectReference{}
	}

	ref := corev1.ObjectReference{
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
	}

	// Extract name and namespace from object metadata
	if metaObj, ok := obj.(metav1.Object); ok {
		ref.Name = metaObj.GetName()
		ref.Namespace = metaObj.GetNamespace()
	}

	return ref
}
