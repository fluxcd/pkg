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
	"github.com/fluxcd/pkg/ssa/jsondiff"
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

	// Force configures the engine to recreate objects that contain immutable field changes.
	Force bool `json:"force"`

	// ForceSelector determines which in-cluster objects are Force applied
	// based on the matching labels or annotations.
	ForceSelector map[string]string `json:"forceSelector"`

	// DriftIgnoreRules specifies field-level ignore rules for drift
	// detection. When set, the specified JSON pointer paths are stripped
	// from both the existing and dry-run objects before comparison.
	DriftIgnoreRules []jsondiff.IgnoreRule `json:"driftIgnoreRules,omitempty"`
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
	if err := m.dryRunApply(ctx, dryRunObject); err != nil {
		if m.shouldForceApply(object, existingObject, ApplyOptions{
			Force:         opts.Force,
			ForceSelector: opts.ForceSelector,
		}, err) {
			return m.changeSetEntry(object, CreatedAction), nil, nil, nil
		}

		return nil, nil, nil, errors.NewDryRunErr(err, dryRunObject)
	}

	if dryRunObject.GetResourceVersion() == "" {
		return m.changeSetEntry(dryRunObject, CreatedAction), nil, nil, nil
	}

	// Compile ignore rules once for drift detection.
	var compiled jsondiff.CompiledIgnoreRules
	if len(opts.DriftIgnoreRules) > 0 {
		var err error
		compiled, err = jsondiff.CompileIgnoreRules(opts.DriftIgnoreRules)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	drifted, err := m.hasDriftedWithIgnore(existingObject, dryRunObject, compiled)
	if err != nil {
		return nil, nil, nil, err
	}

	if drifted {
		cse := m.changeSetEntry(object, ConfiguredAction)

		unstructured.RemoveNestedField(dryRunObject.Object, "metadata", "managedFields")
		unstructured.RemoveNestedField(existingObject.Object, "metadata", "managedFields")

		// Strip ignored fields from the returned objects so the
		// caller's diff output only shows non-ignored changes.
		if compiled != nil {
			if err := removeIgnoredFields(dryRunObject, existingObject, compiled); err != nil {
				return nil, nil, nil, err
			}
			if err := removeIgnoredFields(dryRunObject, dryRunObject, compiled); err != nil {
				return nil, nil, nil, err
			}
		}

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
