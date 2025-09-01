/*
Copyright 2021 Stefan Prodan
Copyright 2021 The Flux authors

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

package ssa

import (
	"context"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/ssa/errors"
	"github.com/fluxcd/pkg/ssa/normalize"
	"github.com/fluxcd/pkg/ssa/utils"
)

// DiffOptions contains options for server-side dry-run apply requests.
type DiffOptions struct {
	// Exclusions determines which in-cluster objects are skipped from dry-run apply
	// based on the matching labels or annotations.
	Exclusions map[string]string `json:"exclusions"`

	// IfNotPresentSelector determines which in-cluster objects are skipped from dry-run apply
	// based on the matching labels or annotations.
	IfNotPresentSelector map[string]string `json:"ifNotPresentSelector"`

	// Strict enables strict field validation, making the server reject
	// requests that contain unknown or duplicate fields.
	// This requires Kubernetes v1.27+.
	// https://kubernetes.io/blog/2023/04/24/openapi-v3-field-validation-ga/#server-side-field-validation
	Strict bool `json:"strict"`
}

// DefaultDiffOptions returns the default dry-run apply options.
func DefaultDiffOptions() DiffOptions {
	return DiffOptions{
		Exclusions: nil,
	}
}

// Diff performs a server-side apply dry-un and returns the live and merged objects if drift is detected.
// If the diff contains Kubernetes Secrets, the data values are masked.
func (m *ResourceManager) Diff(ctx context.Context, object *unstructured.Unstructured, opts DiffOptions) (
	*ChangeSetEntry,
	*unstructured.Unstructured,
	*unstructured.Unstructured,
	error,
) {
	utils.RemoveCABundleFromCRD(object)
	existingObject := &unstructured.Unstructured{}
	existingObject.SetGroupVersionKind(object.GroupVersionKind())
	_ = m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

	if m.shouldSkipDiff(object, existingObject, opts) {
		return m.changeSetEntry(existingObject, SkippedAction), nil, nil, nil
	}

	dryRunObject := object.DeepCopy()
	if err := m.dryRunApply(ctx, dryRunObject, opts.Strict); err != nil {
		return nil, nil, nil, errors.NewDryRunErr(err, dryRunObject)
	}

	if dryRunObject.GetResourceVersion() == "" {
		return m.changeSetEntry(dryRunObject, CreatedAction), nil, nil, nil
	}

	if m.hasDrifted(existingObject, dryRunObject) {
		cse := m.changeSetEntry(object, ConfiguredAction)

		unstructured.RemoveNestedField(dryRunObject.Object, "metadata", "managedFields")
		unstructured.RemoveNestedField(existingObject.Object, "metadata", "managedFields")

		if utils.IsSecret(dryRunObject) {
			if err := SanitizeUnstructuredData(existingObject, dryRunObject); err != nil {
				return nil, nil, nil, err
			}
		}

		return cse, existingObject, dryRunObject, nil
	}

	return m.changeSetEntry(dryRunObject, UnchangedAction), nil, nil, nil
}

// hasDrifted detects changes to metadata labels, annotations and spec.
func (m *ResourceManager) hasDrifted(existingObject, dryRunObject *unstructured.Unstructured) bool {
	if dryRunObject.GetResourceVersion() == "" {
		return true
	}

	if !apiequality.Semantic.DeepEqual(dryRunObject.GetLabels(), existingObject.GetLabels()) {
		return true
	}

	if !apiequality.Semantic.DeepEqual(dryRunObject.GetAnnotations(), existingObject.GetAnnotations()) {
		return true
	}

	return hasObjectDrifted(dryRunObject, existingObject)
}

// hasObjectDrifted performs a semantic equality check of the given objects' spec
func hasObjectDrifted(existingObject, dryRunObject *unstructured.Unstructured) bool {
	existingObj := prepareObjectForDiff(existingObject)
	dryRunObj := prepareObjectForDiff(dryRunObject)

	return !apiequality.Semantic.DeepEqual(dryRunObj.Object, existingObj.Object)
}

// prepareObjectForDiff removes the metadata and status fields from the given object
func prepareObjectForDiff(object *unstructured.Unstructured) *unstructured.Unstructured {
	deepCopy := object.DeepCopy()
	unstructured.RemoveNestedField(deepCopy.Object, "metadata")
	unstructured.RemoveNestedField(deepCopy.Object, "status")
	if err := normalize.DryRunUnstructured(deepCopy); err != nil {
		return object
	}
	return deepCopy
}

// shouldSkipDiff determines based on the object metadata and DiffOptions if the object should be skipped.
// An object is not applied if it contains a label or annotation
// which matches the DiffOptions.Exclusions or DiffOptions.IfNotPresentSelector.
func (m *ResourceManager) shouldSkipDiff(desiredObject *unstructured.Unstructured,
	existingObject *unstructured.Unstructured, opts DiffOptions) bool {
	if utils.AnyInMetadata(desiredObject, opts.Exclusions) ||
		(existingObject != nil && utils.AnyInMetadata(existingObject, opts.Exclusions)) {
		return true
	}

	if existingObject != nil &&
		existingObject.GetUID() != "" &&
		utils.AnyInMetadata(desiredObject, opts.IfNotPresentSelector) {
		return true
	}

	return false
}
