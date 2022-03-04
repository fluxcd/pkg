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
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyOptions contains options for server-side apply requests.
type ApplyOptions struct {
	// Force configures the engine to recreate objects that contain immutable field changes.
	Force bool `json:"force"`

	// Exclusions determines which in-cluster objects are skipped from apply
	// based on the specified key-value pairs.
	// A nil Exclusions map means all objects are applied
	// regardless of their metadata labels and annotations.
	Exclusions map[string]string `json:"exclusions"`

	// WaitTimeout defines after which interval should the engine give up on waiting for
	// cluster scoped resources to become ready.
	WaitTimeout time.Duration `json:"waitTimeout"`

	// Cleanup defines which in-cluster metadata entries are to be removed before applying objects.
	Cleanup ApplyCleanupOptions `json:"cleanup"`
}

// ApplyCleanupOptions defines which metadata entries are to be removed before applying objects.
type ApplyCleanupOptions struct {
	// Annotations defines which 'metadata.annotations' keys should be removed from in-cluster objects.
	Annotations []string `json:"annotations,omitempty"`

	// Labels defines which 'metadata.labels' keys should be removed from in-cluster objects.
	Labels []string `json:"labels,omitempty"`

	// FieldManagers defines which `metadata.managedFields` managers should be removed from in-cluster objects.
	FieldManagers []FieldManager `json:"fieldManagers,omitempty"`

	// Exclusions determines which in-cluster objects are skipped from cleanup
	// based on the specified key-value pairs.
	Exclusions map[string]string `json:"exclusions"`
}

// DefaultApplyOptions returns the default apply options where force apply is disabled.
func DefaultApplyOptions() ApplyOptions {
	return ApplyOptions{
		Force:       false,
		Exclusions:  nil,
		WaitTimeout: 60 * time.Second,
	}
}

// Apply performs a server-side apply of the given object if the matching in-cluster object is different or if it doesn't exist.
// Drift detection is performed by comparing the server-side dry-run result with the existing object.
// When immutable field changes are detected, the object is recreated if 'force' is set to 'true'.
func (m *ResourceManager) Apply(ctx context.Context, object *unstructured.Unstructured, opts ApplyOptions) (*ChangeSetEntry, error) {
	existingObject := object.DeepCopy()
	_ = m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

	if existingObject != nil && AnyInMetadata(existingObject, opts.Exclusions) {
		return m.changeSetEntry(object, UnchangedAction), nil
	}

	dryRunObject := object.DeepCopy()
	if err := m.dryRunApply(ctx, dryRunObject); err != nil {
		if opts.Force && IsImmutableError(err) {
			if err := m.client.Delete(ctx, existingObject); err != nil {
				return nil, fmt.Errorf("%s immutable field detected, failed to delete object, error: %w",
					FmtUnstructured(dryRunObject), err)
			}
			return m.Apply(ctx, object, opts)
		}

		return nil, m.validationError(dryRunObject, err)
	}

	patched, err := m.cleanupMetadata(ctx, object, existingObject, opts.Cleanup)
	if err != nil {
		return nil, fmt.Errorf("%s metadata.managedFields cleanup failed, error: %w",
			FmtUnstructured(existingObject), err)
	}

	// do not apply objects that have not drifted to avoid bumping the resource version
	if !patched && !m.hasDrifted(existingObject, dryRunObject) {
		return m.changeSetEntry(object, UnchangedAction), nil
	}

	appliedObject := object.DeepCopy()
	if err := m.apply(ctx, appliedObject); err != nil {
		return nil, fmt.Errorf("%s apply failed, error: %w", FmtUnstructured(appliedObject), err)
	}

	if dryRunObject.GetResourceVersion() == "" {
		return m.changeSetEntry(appliedObject, CreatedAction), nil
	}

	return m.changeSetEntry(appliedObject, ConfiguredAction), nil
}

// ApplyAll performs a server-side dry-run of the given objects, and based on the diff result,
// it applies the objects that are new or modified.
func (m *ResourceManager) ApplyAll(ctx context.Context, objects []*unstructured.Unstructured, opts ApplyOptions) (*ChangeSet, error) {
	sort.Sort(SortableUnstructureds(objects))
	changeSet := NewChangeSet()
	var toApply []*unstructured.Unstructured
	for _, object := range objects {
		existingObject := object.DeepCopy()
		_ = m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

		if existingObject != nil && AnyInMetadata(existingObject, opts.Exclusions) {
			changeSet.Add(*m.changeSetEntry(existingObject, UnchangedAction))
			continue
		}

		dryRunObject := object.DeepCopy()
		if err := m.dryRunApply(ctx, dryRunObject); err != nil {
			if opts.Force && IsImmutableError(err) {
				if err := m.client.Delete(ctx, existingObject); err != nil {
					return nil, fmt.Errorf("%s immutable field detected, failed to delete object, error: %w",
						FmtUnstructured(dryRunObject), err)
				}
				return m.ApplyAll(ctx, objects, opts)
			}

			return nil, m.validationError(dryRunObject, err)
		}

		patched, err := m.cleanupMetadata(ctx, object, existingObject, opts.Cleanup)
		if err != nil {
			return nil, fmt.Errorf("%s metadata.managedFields cleanup failed, error: %w",
				FmtUnstructured(existingObject), err)
		}

		if patched || m.hasDrifted(existingObject, dryRunObject) {
			toApply = append(toApply, object)
			if dryRunObject.GetResourceVersion() == "" {
				changeSet.Add(*m.changeSetEntry(dryRunObject, CreatedAction))
			} else {
				changeSet.Add(*m.changeSetEntry(dryRunObject, ConfiguredAction))
			}
		} else {
			changeSet.Add(*m.changeSetEntry(dryRunObject, UnchangedAction))
		}
	}

	for _, object := range toApply {
		appliedObject := object.DeepCopy()
		if err := m.apply(ctx, appliedObject); err != nil {
			return nil, fmt.Errorf("%s apply failed, error: %w", FmtUnstructured(appliedObject), err)
		}
	}

	return changeSet, nil
}

// ApplyAllStaged extracts the CRDs and Namespaces, applies them with ApplyAll,
// waits for CRDs and Namespaces to become ready, then is applies all the other objects.
// This function should be used when the given objects have a mix of custom resource definition and custom resources,
// or a mix of namespace definitions with namespaced objects.
func (m *ResourceManager) ApplyAllStaged(ctx context.Context, objects []*unstructured.Unstructured, opts ApplyOptions) (*ChangeSet, error) {
	changeSet := NewChangeSet()

	// contains only CRDs and Namespaces
	var stageOne []*unstructured.Unstructured

	// contains all objects except for CRDs and Namespaces
	var stageTwo []*unstructured.Unstructured

	for _, u := range objects {
		if IsClusterDefinition(u) {
			stageOne = append(stageOne, u)
		} else {
			stageTwo = append(stageTwo, u)
		}
	}

	if len(stageOne) > 0 {
		cs, err := m.ApplyAll(ctx, stageOne, opts)
		if err != nil {
			return nil, err
		}
		changeSet.Append(cs.Entries)

		if err := m.Wait(stageOne, WaitOptions{2 * time.Second, opts.WaitTimeout}); err != nil {
			return nil, err
		}
	}

	cs, err := m.ApplyAll(ctx, stageTwo, opts)
	if err != nil {
		return nil, err
	}
	changeSet.Append(cs.Entries)

	return changeSet, nil
}

func (m *ResourceManager) dryRunApply(ctx context.Context, object *unstructured.Unstructured) error {
	opts := []client.PatchOption{
		client.DryRunAll,
		client.ForceOwnership,
		client.FieldOwner(m.owner.Field),
	}
	return m.client.Patch(ctx, object, client.Apply, opts...)
}

func (m *ResourceManager) apply(ctx context.Context, object *unstructured.Unstructured) error {
	opts := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(m.owner.Field),
	}
	return m.client.Patch(ctx, object, client.Apply, opts...)
}

// cleanupMetadata performs an HTTP PATCH request to remove entries from metadata annotations, labels and managedFields.
func (m *ResourceManager) cleanupMetadata(ctx context.Context,
	desiredObject *unstructured.Unstructured,
	object *unstructured.Unstructured,
	opts ApplyCleanupOptions) (bool, error) {
	if AnyInMetadata(desiredObject, opts.Exclusions) || AnyInMetadata(object, opts.Exclusions) {
		return false, nil
	}

	if object == nil {
		return false, nil
	}
	existingObject := object.DeepCopy()
	var patches []jsonPatch

	if len(opts.Annotations) > 0 {
		patches = append(patches, patchRemoveAnnotations(existingObject, opts.Annotations)...)
	}

	if len(opts.Labels) > 0 {
		patches = append(patches, patchRemoveLabels(existingObject, opts.Labels)...)
	}

	if len(opts.FieldManagers) > 0 {
		managedFieldPatch, err := patchReplaceFieldsManagers(existingObject, opts.FieldManagers, m.owner.Field)
		if err != nil {
			return false, err
		}
		patches = append(patches, managedFieldPatch...)
	}

	// no patching is needed exit early
	if len(patches) == 0 {
		return false, nil
	}

	rawPatch, err := json.Marshal(patches)
	if err != nil {
		return false, err
	}
	patch := client.RawPatch(types.JSONPatchType, rawPatch)

	return true, m.client.Patch(ctx, existingObject, patch, client.FieldOwner(m.owner.Field))
}
