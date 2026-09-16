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

package v1

// These constants define common event actions used throughout Flux controllers.
const (
	// ActionReconciling indicates a reconciliation is in progress.
	ActionReconciling string = "Reconciling"
	// ActionReconciled indicates a successful reconciliation.
	ActionReconciled string = "Reconciled"
	// ActionFetching indicates fetching of a resource or artifact.
	ActionFetching string = "Fetching"
	// ActionFetched indicates successful fetch of a resource or artifact.
	ActionFetched string = "Fetched"
	// ActionApplying indicates applying changes to the cluster.
	ActionApplying string = "Applying"
	// ActionApplied indicates successful application of changes.
	ActionApplied string = "Applied"
	// ActionDeleting indicates deletion is in progress.
	ActionDeleting string = "Deleting"
	// ActionDeleted indicates successful deletion.
	ActionDeleted string = "Deleted"
	// ActionValidating indicates validation is in progress.
	ActionValidating string = "Validating"
	// ActionValidated indicates successful validation.
	ActionValidated string = "Validated"
	// ActionWaiting indicates waiting for a condition.
	ActionWaiting string = "Waiting"
	// ActionProgressing indicates progression through a workflow.
	ActionProgressing string = "Progressing"
	// ActionFailed indicates a failed operation.
	ActionFailed string = "Failed"
)
