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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ReadyCondition indicates the resource is ready and fully reconciled.
	// If the Condition is False, the resource should be considered to be in the process
	// of reconciling and not a representation of actual state.
	ReadyCondition string = "Ready"

	// StalledCondition indicates the reconciliation of the resource has stalled, e.g.
	// because the controller has encountered an error during the reconcile process or
	// it has made insufficient progress (timeout).
	// The Condition adheres to an "abnormal-true" polarity pattern, and MUST only be
	// present on the resource if the Condition is True.
	StalledCondition string = "Stalled"

	// ReconcilingCondition indicates the controller is currently working on reconciling the
	// latest changes. This MAY be True for multiple reconciliation attempts, e.g. when an
	// transient error occurred.
	// The Condition adheres to an "abnormal-true" polarity pattern, and MUST only be
	// present on the resource if the Condition is True.
	ReconcilingCondition string = "Reconciling"
)

const (
	// SucceededReason represents the fact that the reconciliation of
	// a toolkit resource has succeeded.
	SucceededReason string = "Succeeded"

	// FailedReason represents the fact that the reconciliation of a
	// toolkit resource has failed.
	FailedReason string = "Failed"

	// ProgressingReason represents the fact that the reconciliation of a toolkit
	// resource is underway.
	ProgressingReason string = "Progressing"

	// SuspendedReason represents the fact that the reconciliation of a toolkit
	// resource is suspended.
	SuspendedReason string = "Suspended"
)

// ObjectWithConditions is an interface that describes Kubernetes API type
// structs that have status conditions.
// +k8s:deepcopy-gen=false
type ObjectWithConditions interface {
	GetConditions() []metav1.Condition
}

// ObjectWithConditionsSetter is an interface that describes Kubernetes API type
// structs that have a status conditions setter.
// +k8s:deepcopy-gen=false
type ObjectWithConditionsSetter interface {
	SetConditions([]metav1.Condition)
}
