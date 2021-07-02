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

const (
	// ReconcileAtAnnotation is the annotation used for triggering a reconciliation
	// outside of the defined schedule. Despite the name, the value is not
	// interpreted as a timestamp, and any change in value shall trigger a
	// reconciliation.
	// DEPRECATED: has been replaced by ReconcileRequestAnnotation. For
	// backward-compatibility, use ReconcileAnnotationValue, which will account for
	// both annotations.
	ReconcileAtAnnotation string = "fluxcd.io/reconcileAt"

	// ReconcileRequestAnnotation is the annotation used for triggering a reconciliation
	// outside of a defined schedule. The value is interpreted as a token, and any change
	// in value SHOULD trigger a reconciliation.
	ReconcileRequestAnnotation string = "reconcile.fluxcd.io/requestedAt"
)

// ReconcileAnnotationValue returns a value for the reconciliation request annotations, which can be used to detect
// changes; and, a boolean indicating whether either annotation was set.
func ReconcileAnnotationValue(annotations map[string]string) (string, bool) {
	reconcileAt, ok1 := annotations[ReconcileAtAnnotation]
	requestedAt, ok2 := annotations[ReconcileRequestAnnotation]
	// the values are concatenated; this means
	// - a change in either will be detectable*, and
	// - if one is set, the value will be just that; and,
	// - if neither is set, it's a zero value.
	//
	// *unless the change is to shift a substring across the
	// interstice between the strings; e.g., by swapping the value
	// from one annotation to the other. Assuming a fresh timestamp is
	// used each time, this caveat won't matter.
	return reconcileAt + requestedAt, ok1 || ok2
}

// ReconcileRequestStatus is a struct to embed in a status type, so that all types using the mechanism have the same
// field. Use it like this:
//
//	type FooStatus struct {
//  	meta.ReconcileRequestStatus `json:",inline"`
//  	// other status fields...
// 	}
type ReconcileRequestStatus struct {
	// LastHandledReconcileAt holds the value of the most recent
	// reconcile request value, so a change of the annotation value
	// can be detected.
	// +optional
	LastHandledReconcileAt string `json:"lastHandledReconcileAt,omitempty"`
}

// GetLastHandledReconcileRequest returns the most recent reconcile request value from the ReconcileRequestStatus.
func (in ReconcileRequestStatus) GetLastHandledReconcileRequest() string {
	return in.LastHandledReconcileAt
}

// SetLastHandledReconcileRequest sets the most recent reconcile request value in the ReconcileRequestStatus.
func (in *ReconcileRequestStatus) SetLastHandledReconcileRequest(token string) {
	in.LastHandledReconcileAt = token
}

// StatusWithHandledReconcileRequest describes a status type which holds the value of the most recent
// ReconcileAnnotationValue.
// +k8s:deepcopy-gen=false
type StatusWithHandledReconcileRequest interface {
	GetLastHandledReconcileRequest() string
}

// StatusWithHandledReconcileRequestSetter describes a status with a setter for the most ReconcileAnnotationValue.
// +k8s:deepcopy-gen=false
type StatusWithHandledReconcileRequestSetter interface {
	SetLastHandledReconcileRequest(token string)
}
