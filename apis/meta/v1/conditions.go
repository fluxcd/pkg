/*
Copyright 2020 The Flux CD contributors.

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

package v1

const (
	// ReadyCondition is the name of the Ready condition
	// implemented by all toolkit resources.
	ReadyCondition string = "Ready"

	// ReconciliationSucceededReason represents the fact that the
	// reconciliation of a toolkit resource has succeeded.
	ReconciliationSucceededReason string = "ReconciliationSucceeded"

	// ReconciliationFailedReason represents the fact that the
	// reconciliation of a toolkit resource has failed.
	ReconciliationFailedReason string = "ReconciliationFailed"

	// ProgressingReason represents the fact that the
	// reconciliation of a toolkit resource is underway.
	ProgressingReason string = "Progressing"

	// DependencyNotReadyReason represents the fact that
	// one of the toolkit resource dependencies is not ready.
	DependencyNotReadyReason string = "DependencyNotReady"

	// SuspendedReason represents the fact that the
	// reconciliation of a toolkit resource is suspended.
	SuspendedReason string = "Suspended"
)
