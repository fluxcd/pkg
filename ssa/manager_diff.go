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
	"fmt"
	"strings"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultMask = "*****"
	diffMask    = "******"
)

// DiffOptions contains options for server-side dry-run apply requests.
type DiffOptions struct {
	// Exclusions determines which in-cluster objects are skipped from dry-run apply
	// based on the specified key-value pairs.
	// A nil Exclusions map means all objects are applied
	// regardless of their metadata labels and annotations.
	Exclusions map[string]string `json:"exclusions"`
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
	existingObject := object.DeepCopy()
	_ = m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

	if existingObject != nil && AnyInMetadata(existingObject, opts.Exclusions) {
		return m.changeSetEntry(existingObject, UnchangedAction), nil, nil, nil
	}

	dryRunObject := object.DeepCopy()
	if err := m.dryRunApply(ctx, dryRunObject); err != nil {
		return nil, nil, nil, m.validationError(dryRunObject, err)
	}

	if dryRunObject.GetResourceVersion() == "" {
		return m.changeSetEntry(dryRunObject, CreatedAction), nil, nil, nil
	}

	if m.hasDrifted(existingObject, dryRunObject) {
		cse := m.changeSetEntry(object, ConfiguredAction)

		unstructured.RemoveNestedField(dryRunObject.Object, "metadata", "managedFields")
		unstructured.RemoveNestedField(existingObject.Object, "metadata", "managedFields")

		if dryRunObject.GetKind() == "Secret" {
			d, ex, err := m.sanitizeDriftedSecrets(existingObject, dryRunObject)
			if err != nil {
				return nil, nil, nil, err
			}

			dryRunObject, existingObject = d, ex
		}

		return cse, existingObject, dryRunObject, nil
	}

	return m.changeSetEntry(dryRunObject, UnchangedAction), nil, nil, nil
}

// sanitizeDriftedSecrets masks the data values of the given secret objects
func (m *ResourceManager) sanitizeDriftedSecrets(existingObject, dryRunObject *unstructured.Unstructured) (*unstructured.Unstructured, *unstructured.Unstructured, error) {
	dryRunData, foundDryRun, err := getNestedMap(dryRunObject)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get data from dry run object, error: %w", err)
	}

	existingData, foundExisting, err := getNestedMap(existingObject)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get data from existing object, error: %w", err)
	}

	if !foundDryRun || !foundExisting {
		if foundDryRun {
			d, err := maskSecret(dryRunData, dryRunObject, diffMask)
			if err != nil {
				return nil, nil, fmt.Errorf("masking secret data failed, error: %w", err)
			}
			return d, existingObject, nil
		}

		e, err := maskSecret(existingData, existingObject, diffMask)
		if err != nil {
			return nil, nil, fmt.Errorf("masking secret data failed, error: %w", err)
		}
		return dryRunObject, e, nil
	}

	if foundDryRun && foundExisting {
		d, ex := cmpMaskData(dryRunData, existingData)

		err := setNestedMap(dryRunObject, d)
		if err != nil {
			return nil, nil, fmt.Errorf("masking secret data failed, error: %w", err)
		}

		err = setNestedMap(existingObject, ex)
		if err != nil {
			return nil, nil, fmt.Errorf("masking secret data failed, error: %w", err)
		}

	}

	return dryRunObject, existingObject, nil

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
	if err := fixHorizontalPodAutoscaler(deepCopy); err != nil {
		return object
	}
	return deepCopy
}

// validationError formats the given error and hides sensitive data
// if the error was caused by an invalid Kubernetes secrets.
func (m *ResourceManager) validationError(object *unstructured.Unstructured, err error) error {
	if apierrors.IsNotFound(err) {
		return fmt.Errorf("%s namespace not specified, error: %w", FmtUnstructured(object), err)
	}

	reason := fmt.Sprintf("%v", apierrors.ReasonForError(err))

	if object.GetKind() == "Secret" {
		msg := "data values must be of type string"
		if strings.Contains(err.Error(), "immutable") {
			msg = "secret is immutable"
		}
		return fmt.Errorf("%s %s, error: %s", FmtUnstructured(object), strings.ToLower(reason), msg)
	}

	// detect managed field conflict
	if status, ok := apierrors.StatusCause(err, metav1.CauseTypeFieldManagerConflict); ok {
		reason = fmt.Sprintf("%v", status.Type)
	}

	if reason != "" {
		reason = fmt.Sprintf(", reason: %s", reason)
	}

	return fmt.Errorf("%s dry-run failed%s, error: %w",
		FmtUnstructured(object), reason, err)

}
