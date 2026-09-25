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

package testenv

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetEvents lists corev1.Event objects from the apiserver that match the given
// object name. If namespace is non-empty, events are also filtered by the
// involved object's namespace. If annotations is non-empty, only events whose
// annotations contain at least one matching key-value pair are returned. It
// returns an error if listing events from the client fails.
//
// It reads core/v1 Events because that is the representation the events
// Recorder writes: the legacy core/v1 event recorder does not set EventTime and
// therefore is not subject to the apiserver's strict note-length validation.
func GetEvents(ctx context.Context, client client.Client, objName, namespace string, annotations map[string]string) ([]corev1.Event, error) {
	var result []corev1.Event
	events := &corev1.EventList{}
	if err := client.List(ctx, events); err != nil {
		return nil, err
	}
	for _, event := range events.Items {
		if event.InvolvedObject.Name != objName {
			continue
		}
		if namespace != "" && event.InvolvedObject.Namespace != namespace {
			continue
		}
		if len(annotations) == 0 {
			result = append(result, event)
			continue
		}
		for ak, av := range annotations {
			if event.GetAnnotations()[ak] == av {
				result = append(result, event)
				break
			}
		}
	}
	return result, nil
}

// WaitForEvents polls the apiserver until at least minCount corev1.Event objects
// matching the given object name (and optional namespace and annotations) are
// found, or the timeout elapses. It returns the matching events.
//
// Kubernetes events are recorded asynchronously, so a controller emitting an
// event does not guarantee it is immediately queryable. More importantly, the
// apiserver may reject an event during admission (for example, a message longer
// than the server-enforced limit on the strict events path) without surfacing
// an error to the recorder. In that case the event never lands and this helper
// times out, turning a silently dropped event into a test failure. Use it to
// assert that events a controller emits actually survive the round-trip to the
// apiserver.
func WaitForEvents(ctx context.Context, c client.Client, objName, namespace string, annotations map[string]string, minCount int, timeout time.Duration) ([]corev1.Event, error) {
	if minCount < 1 {
		minCount = 1
	}
	var result []corev1.Event
	err := wait.PollUntilContextTimeout(ctx, 100*time.Millisecond, timeout, true, func(ctx context.Context) (bool, error) {
		events, err := GetEvents(ctx, c, objName, namespace, annotations)
		if err != nil {
			return false, err
		}
		result = events
		return len(events) >= minCount, nil
	})
	if err != nil {
		return result, fmt.Errorf("waiting for at least %d event(s) for object %q: %w (found %d)", minCount, objName, err, len(result))
	}
	return result, nil
}
