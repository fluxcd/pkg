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
	"testing"

	eventsv1 "k8s.io/api/events/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetEvents(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = eventsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	events := []client.Object{
		&eventsv1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "event-1",
				Namespace: "default",
				Annotations: map[string]string{
					"revision": "v1",
				},
			},
			Regarding: corev1.ObjectReference{
				Name:      "my-kustomization",
				Namespace: "default",
			},
		},
		&eventsv1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "event-2",
				Namespace: "default",
			},
			Regarding: corev1.ObjectReference{
				Name:      "my-kustomization",
				Namespace: "default",
			},
		},
		&eventsv1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "event-3",
				Namespace: "other-ns",
			},
			Regarding: corev1.ObjectReference{
				Name:      "my-kustomization",
				Namespace: "other-ns",
			},
		},
		&eventsv1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "event-4",
				Namespace: "default",
			},
			Regarding: corev1.ObjectReference{
				Name:      "other-obj",
				Namespace: "default",
			},
		},
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(events...).Build()
	ctx := context.Background()

	tests := []struct {
		name        string
		objName     string
		namespace   string
		annotations map[string]string
		wantCount   int
	}{
		{
			name:      "filter by name only",
			objName:   "my-kustomization",
			wantCount: 3,
		},
		{
			name:      "filter by name and namespace",
			objName:   "my-kustomization",
			namespace: "default",
			wantCount: 2,
		},
		{
			name:        "filter by name and annotation",
			objName:     "my-kustomization",
			annotations: map[string]string{"revision": "v1"},
			wantCount:   1,
		},
		{
			name:      "no match",
			objName:   "nonexistent",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEvents(ctx, c, tt.objName, tt.namespace, tt.annotations)
			if len(got) != tt.wantCount {
				t.Errorf("GetEvents() returned %d events, want %d", len(got), tt.wantCount)
			}
		})
	}
}
