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

// Diff performs a server-side apply dry-un and returns the live and merged objects if drift is detected.
// If the diff contains Kubernetes Secrets, the data values are masked.
func (m *ResourceManager) Diff(ctx context.Context, object *unstructured.Unstructured) (*ChangeSetEntry, *unstructured.Unstructured, *unstructured.Unstructured, error) {
	existingObject := object.DeepCopy()
	_ = m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

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
			d, err := MaskSecret(dryRunObject, "******")
			if err != nil {
				return nil, nil, nil, fmt.Errorf("masking secret data failed, error: %w", err)
			}
			dryRunObject = d
			ex, err := MaskSecret(existingObject, "*****")
			if err != nil {
				return nil, nil, nil, fmt.Errorf("masking secret data failed, error: %w", err)
			}
			existingObject = ex
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

	if !apiequality.Semantic.DeepDerivative(dryRunObject.GetLabels(), existingObject.GetLabels()) {
		return true

	}

	if !apiequality.Semantic.DeepDerivative(dryRunObject.GetAnnotations(), existingObject.GetAnnotations()) {
		return true
	}

	return hasObjectDrifted(dryRunObject, existingObject)
}

// hasObjectDrifted removes the metadata and status fields from both objects
// then performs a semantic equality check of the remaining fields
func hasObjectDrifted(dryRunObject, existingObject *unstructured.Unstructured) bool {
	dryRunObj := dryRunObject.DeepCopy()
	unstructured.RemoveNestedField(dryRunObj.Object, "metadata")
	unstructured.RemoveNestedField(dryRunObj.Object, "status")

	existingObj := existingObject.DeepCopy()
	unstructured.RemoveNestedField(existingObj.Object, "metadata")
	unstructured.RemoveNestedField(existingObj.Object, "status")

	return !apiequality.Semantic.DeepEqual(dryRunObj.Object, existingObj.Object)
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
