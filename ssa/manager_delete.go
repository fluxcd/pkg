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
	"sort"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/ssa/utils"
)

// DeleteOptions contains options for delete requests.
type DeleteOptions struct {
	// PropagationPolicy determined whether and how garbage collection will be
	// performed.
	PropagationPolicy metav1.DeletionPropagation

	// Inclusions determines which in-cluster objects are subject to deletion
	// based on the specified key-value pairs.
	// A nil Inclusions map means all objects are subject to deletion
	// irregardless of their metadata labels.
	Inclusions map[string]string

	// Exclusions determines which in-cluster objects are skipped from deletion
	// based on the specified key-value pairs.
	// A nil Exclusions map means all objects are subject to deletion
	// irregardless of their metadata labels and annotations.
	Exclusions map[string]string
}

// DefaultDeleteOptions returns the default delete options where the propagation
// policy is set to background.
func DefaultDeleteOptions() DeleteOptions {
	return DeleteOptions{
		PropagationPolicy: metav1.DeletePropagationBackground,
		Inclusions:        nil,
		Exclusions:        nil,
	}
}

// Delete deletes the given object (not found errors are ignored).
func (m *ResourceManager) Delete(ctx context.Context, object *unstructured.Unstructured, opts DeleteOptions) (*ChangeSetEntry, error) {

	existingObject := &unstructured.Unstructured{}
	existingObject.SetGroupVersionKind(object.GroupVersionKind())
	err := m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return m.changeSetEntry(object, UnknownAction),
				fmt.Errorf("%s query failed: %w", utils.FmtUnstructured(object), err)
		}
		return m.changeSetEntry(object, DeletedAction), nil
	}

	sel, err := metav1.LabelSelectorAsSelector(&metav1.LabelSelector{MatchLabels: opts.Inclusions})
	if err != nil {
		return m.changeSetEntry(object, UnknownAction),
			fmt.Errorf("%s label selector failed: %w", utils.FmtUnstructured(object), err)
	}

	if !sel.Matches(labels.Set(existingObject.GetLabels())) {
		return m.changeSetEntry(object, SkippedAction), nil
	}

	if utils.AnyInMetadata(existingObject, opts.Exclusions) {
		return m.changeSetEntry(object, SkippedAction), nil
	}

	if err := m.client.Delete(ctx, existingObject, client.PropagationPolicy(opts.PropagationPolicy)); err != nil {
		return m.changeSetEntry(object, UnknownAction),
			fmt.Errorf("%s delete failed: %w", utils.FmtUnstructured(object), err)
	}

	return m.changeSetEntry(object, DeletedAction), nil
}

// DeleteAll deletes the given set of objects (not found errors are ignored).
func (m *ResourceManager) DeleteAll(ctx context.Context, objects []*unstructured.Unstructured, opts DeleteOptions) (*ChangeSet, error) {
	sort.Sort(sort.Reverse(SortableUnstructureds(objects)))
	changeSet := NewChangeSet()

	var errors string
	for _, object := range objects {
		cse, err := m.Delete(ctx, object, opts)
		if cse != nil {
			changeSet.Add(*cse)
		}
		if err != nil {
			errors += err.Error() + ";"
		}
	}

	if errors != "" {
		return changeSet, fmt.Errorf("delete failed, errors: %s", errors)
	}

	return changeSet, nil
}
