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
}

type whatever struct {
	Annotations map[string]string
	Status      whateverStatus `json:"status,omitempty"`
}

func TestGetAnnotationValue(t *testing.T) {
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
