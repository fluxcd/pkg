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

package meta

import (
	"testing"
	"time"
)

type whateverStatus struct {
	ReconcileRequestStatus `json:",inline"`
	ForceRequestStatus     `json:",inline"`
}

type whatever struct {
	Annotations map[string]string
	Status      whateverStatus `json:"status"`
}

func (w *whatever) GetAnnotations() map[string]string {
	return w.Annotations
}

func (w *whatever) GetLastHandledReconcileRequest() string {
	return w.Status.GetLastHandledReconcileRequest()
}

func (w *whatever) GetLastHandledForceRequestStatus() *string {
	return &w.Status.LastHandledForceAt
}

func TestGetReconcileAnnotationValue(t *testing.T) {
	obj := whatever{
		Annotations: map[string]string{},
	}

	val, ok := ReconcileAnnotationValue(obj.Annotations)
	if val != "" || ok {
		t.Error("expected ReconcileAnnotationValue to return zero value and false when no annotations")
	}
	obj.Status.SetLastHandledReconcileRequest(val)

	// set annotation: should detect a change
	obj.Annotations[ReconcileRequestAnnotation] = time.Now().Format(time.RFC3339Nano)
	val, ok = ReconcileAnnotationValue(obj.Annotations)
	if !ok {
		t.Error("expected ReconcileAnnotationValue to return true when an annotation is set")
	}

	if val == obj.Status.GetLastHandledReconcileRequest() {
		t.Error("expected to detect change in annotation value")
	}

	obj.Status.SetLastHandledReconcileRequest(val)

	// update annotation; should detect a change
	obj.Annotations[ReconcileRequestAnnotation] = time.Now().Format(time.RFC3339Nano)
	val, ok = ReconcileAnnotationValue(obj.Annotations)
	if !ok {
		t.Error("expected ReconcileAnnotationValue to return true when an annotation is set")
	}

	if val == obj.Status.GetLastHandledReconcileRequest() {
		t.Error("expected to detect change in annotation value")
	}
}

func TestShouldHandleForceRequest(t *testing.T) {
	obj := &whatever{
		Annotations: map[string]string{
			ReconcileRequestAnnotation: "b",
			ForceRequestAnnotation:     "b",
		},
		Status: whateverStatus{
			ReconcileRequestStatus: ReconcileRequestStatus{
				LastHandledReconcileAt: "a",
			},
			ForceRequestStatus: ForceRequestStatus{
				LastHandledForceAt: "a",
			},
		},
	}

	if !ShouldHandleForceRequest(obj) {
		t.Error("ShouldHandleForceRequest() = false")
	}

	if obj.Status.LastHandledForceAt != "b" {
		t.Error("ShouldHandleForceRequest did not update LastHandledForceAt")
	}
}

func TestHandleAnnotationRequest(t *testing.T) {
	const requestAnnotation = "requestAnnotation"

	tests := []struct {
		name                     string
		annotations              map[string]string
		lastHandledReconcile     string
		lastHandledRequest       string
		want                     bool
		expectLastHandledRequest string
	}{
		{
			name: "valid request and reconcile annotations",
			annotations: map[string]string{
				ReconcileRequestAnnotation: "b",
				requestAnnotation:          "b",
			},
			want:                     true,
			expectLastHandledRequest: "b",
		},
		{
			name: "mismatched annotations",
			annotations: map[string]string{
				ReconcileRequestAnnotation: "b",
				requestAnnotation:          "c",
			},
			want:                     false,
			expectLastHandledRequest: "c",
		},
		{
			name: "reconcile matches previous request",
			annotations: map[string]string{
				ReconcileRequestAnnotation: "b",
				requestAnnotation:          "b",
			},
			lastHandledReconcile:     "a",
			lastHandledRequest:       "b",
			want:                     false,
			expectLastHandledRequest: "b",
		},
		{
			name: "request matches previous reconcile",
			annotations: map[string]string{
				ReconcileRequestAnnotation: "b",
				requestAnnotation:          "b",
			},
			lastHandledReconcile:     "b",
			lastHandledRequest:       "a",
			want:                     false,
			expectLastHandledRequest: "b",
		},
		{
			name:                     "missing annotations",
			annotations:              map[string]string{},
			lastHandledRequest:       "a",
			want:                     false,
			expectLastHandledRequest: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &whatever{
				Annotations: tt.annotations,
				Status: whateverStatus{
					ReconcileRequestStatus: ReconcileRequestStatus{
						LastHandledReconcileAt: tt.lastHandledReconcile,
					},
				},
			}

			lastHandled := tt.lastHandledRequest
			result := HandleAnnotationRequest(obj, requestAnnotation, &lastHandled)

			if result != tt.want {
				t.Errorf("HandleAnnotationRequest() = %v, want %v", result, tt.want)
			}
			if lastHandled != tt.expectLastHandledRequest {
				t.Errorf("lastHandledRequest = %v, want %v", lastHandled, tt.expectLastHandledRequest)
			}
		})
	}
}
