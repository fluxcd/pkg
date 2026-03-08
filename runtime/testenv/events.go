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

	eventsv1 "k8s.io/api/events/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetEvents lists eventsv1.Event objects from the apiserver that match the
// given object name. If namespace is non-empty, events are also filtered by
// the regarding object's namespace. If annotations is non-empty, only events
// whose annotations contain at least one matching key-value pair are returned.
func GetEvents(ctx context.Context, client client.Client, objName, namespace string, annotations map[string]string) []eventsv1.Event {
	var result []eventsv1.Event
	events := &eventsv1.EventList{}
	_ = client.List(ctx, events)
	for _, event := range events.Items {
		if event.Regarding.Name != objName {
			continue
		}
		if namespace != "" && event.Regarding.Namespace != namespace {
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
	return result
}
