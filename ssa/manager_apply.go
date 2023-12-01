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

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssaerrors "github.com/fluxcd/pkg/ssa/errors"
	"github.com/fluxcd/pkg/ssa/utils"
)

// ApplyOptions contains options for server-side apply requests.
type ApplyOptions struct {
	// Force configures the engine to recreate objects that contain immutable field changes.
	Force bool `json:"force"`

	// ForceSelector determines which in-cluster objects are Force applied
	// based on the matching labels or annotations.
	ForceSelector map[string]string `json:"forceSelector"`

	// ExclusionSelector determines which in-cluster objects are skipped from apply
	// based on the matching labels or annotations.
	ExclusionSelector map[string]string `json:"exclusionSelector"`

	// IfNotPresentSelector determines which in-cluster objects are skipped from patching
	// based on the matching labels or annotations.
	IfNotPresentSelector map[string]string `json:"ifNotPresentSelector"`

	// WaitInterval defines the interval at which the engine polls for cluster
	// scoped resources to reach their final state.
	WaitInterval time.Duration `json:"waitInterval"`

	// WaitTimeout defines after which interval should the engine give up on waiting for
	// cluster scoped resources to reach their final state.
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
		Force:             false,
		ExclusionSelector: nil,
		WaitInterval:      2 * time.Second,
		WaitTimeout:       60 * time.Second,
	}
}

// Apply performs a server-side apply of the given object if the matching in-cluster object is different or if it doesn't exist.
// Drift detection is performed by comparing the server-side dry-run result with the existing object.
// When immutable field changes are detected, the object is recreated if 'force' is set to 'true'.
func (m *ResourceManager) Apply(ctx context.Context, object *unstructured.Unstructured, opts ApplyOptions) (*ChangeSetEntry, error) {
	existingObject := &unstructured.Unstructured{}
	existingObject.SetGroupVersionKind(object.GroupVersionKind())
	getError := m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

	if m.shouldSkipApply(object, existingObject, opts) {
		return m.changeSetEntry(object, SkippedAction), nil
	}

	dryRunObject := object.DeepCopy()
	if err := m.dryRunApply(ctx, dryRunObject); err != nil {
		if !errors.IsNotFound(getError) && m.shouldForceApply(object, existingObject, opts, err) {
			if err := m.client.Delete(ctx, existingObject, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !errors.IsNotFound(err) {
				return nil, fmt.Errorf("%s immutable field detected, failed to delete object: %w",
					utils.FmtUnstructured(dryRunObject), err)
			}
			return m.Apply(ctx, object, opts)
		}

		return nil, ssaerrors.NewDryRunErr(err, dryRunObject)
	}

	patched, err := m.cleanupMetadata(ctx, object, existingObject, opts.Cleanup)
	if err != nil {
		return nil, fmt.Errorf("%s metadata.managedFields cleanup failed: %w",
			utils.FmtUnstructured(existingObject), err)
	}

	// do not apply objects that have not drifted to avoid bumping the resource version
	if !patched && !m.hasDrifted(existingObject, dryRunObject) {
		return m.changeSetEntry(object, UnchangedAction), nil
	}

	appliedObject := object.DeepCopy()
	if err := m.apply(ctx, appliedObject); err != nil {
		return nil, fmt.Errorf("%s apply failed: %w", utils.FmtUnstructured(appliedObject), err)
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

	// Results are written to the following arrays from the concurrent goroutines. We use arrays
	// to avoid complex synchronization. toApply is sparse, slots are only popuplated when there
	// is an object to apply
	toApply := make([]*unstructured.Unstructured, len(objects))
	changes := make([]ChangeSetEntry, len(objects))

	{
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(m.concurrency)
		for i, object := range objects {
			i, object := i, object

			g.Go(func() error {
				existingObject := &unstructured.Unstructured{}
				existingObject.SetGroupVersionKind(object.GroupVersionKind())
				getError := m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

				if m.shouldSkipApply(object, existingObject, opts) {
					changes[i] = *m.changeSetEntry(existingObject, SkippedAction)
					return nil
				}

				dryRunObject := object.DeepCopy()
				if err := m.dryRunApply(ctx, dryRunObject); err != nil {
					// We cannot have an immutable error (and therefore shouldn't force-apply) if the resource doesn't
					// exist on the cluster. Note that resource might not exist because we wrongly identified an error
					// as immutable and deleted it when ApplyAll was called the last time (the check for ImmutableError
					// returns false positives)
					if !errors.IsNotFound(getError) && m.shouldForceApply(object, existingObject, opts, err) {
						if err := m.client.Delete(ctx, existingObject, client.PropagationPolicy(metav1.DeletePropagationBackground)); err != nil && !errors.IsNotFound(err) {
							return fmt.Errorf("%s immutable field detected, failed to delete object: %w",
								utils.FmtUnstructured(dryRunObject), err)
						}

						// Wait until deleted (in case of any finalizers).
						err = wait.PollUntilContextCancel(ctx, opts.WaitInterval, true, func(ctx context.Context) (bool, error) {
							err := m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)
							if err != nil && errors.IsNotFound(err) {
								// Object has been deleted.
								return true, nil
							}
							// Object still exists, or we got another error than NotFound.
							return false, err
						})
						if err != nil {
							return fmt.Errorf("%s immutable field detected, failed to wait for object to be deleted: %w",
								utils.FmtUnstructured(dryRunObject), err)
						}

						err = m.dryRunApply(ctx, dryRunObject)
					}

					if err != nil {
						return ssaerrors.NewDryRunErr(err, dryRunObject)
					}
				}

				patched, err := m.cleanupMetadata(ctx, object, existingObject, opts.Cleanup)
				if err != nil {
					return fmt.Errorf("%s metadata.managedFields cleanup failed: %w",
						utils.FmtUnstructured(existingObject), err)
				}

				if patched || m.hasDrifted(existingObject, dryRunObject) {
					toApply[i] = object
					if dryRunObject.GetResourceVersion() == "" {
						changes[i] = *m.changeSetEntry(dryRunObject, CreatedAction)
					} else {
						changes[i] = *m.changeSetEntry(dryRunObject, ConfiguredAction)
					}
				} else {
					changes[i] = *m.changeSetEntry(dryRunObject, UnchangedAction)
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, err
		}
	}

	for _, object := range toApply {
		if object != nil {
			appliedObject := object.DeepCopy()
			if err := m.apply(ctx, appliedObject); err != nil {
				return nil, fmt.Errorf("%s apply failed: %w", utils.FmtUnstructured(appliedObject), err)
			}
		}
	}

	changeSet := NewChangeSet()
	changeSet.Append(changes)

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
		if utils.IsClusterDefinition(u) {
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

		if err := m.Wait(stageOne, WaitOptions{opts.WaitInterval, opts.WaitTimeout, false}); err != nil {
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
	if utils.AnyInMetadata(desiredObject, opts.Exclusions) || utils.AnyInMetadata(object, opts.Exclusions) {
		return false, nil
	}

	if object == nil {
		return false, nil
	}
	existingObject := object.DeepCopy()
	var patches []jsonPatch

	if len(opts.Annotations) > 0 {
		patches = append(patches, PatchRemoveAnnotations(existingObject, opts.Annotations)...)
	}

	if len(opts.Labels) > 0 {
		patches = append(patches, PatchRemoveLabels(existingObject, opts.Labels)...)
	}

	if len(opts.FieldManagers) > 0 {
		managedFieldPatch, err := PatchReplaceFieldsManagers(existingObject, opts.FieldManagers, m.owner.Field)
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

// shouldForceApply determines based on the apply error and ApplyOptions if the object should be recreated.
// An object is recreated if the apply error was due to immutable field changes and if the object
// contains a label or annotation which matches the ApplyOptions.ForceSelector.
func (m *ResourceManager) shouldForceApply(desiredObject *unstructured.Unstructured,
	existingObject *unstructured.Unstructured, opts ApplyOptions, err error) bool {
	if ssaerrors.IsImmutableError(err) {
		if opts.Force ||
			utils.AnyInMetadata(desiredObject, opts.ForceSelector) ||
			(existingObject != nil && utils.AnyInMetadata(existingObject, opts.ForceSelector)) {
			return true
		}
	}

	return false
}

// shouldSkipApply determines based on the object metadata and ApplyOptions if the object should be skipped.
// An object is not applied if it contains a label or annotation
// which matches the ApplyOptions.ExclusionSelector or ApplyOptions.IfNotPresentSelector.
func (m *ResourceManager) shouldSkipApply(desiredObject *unstructured.Unstructured,
	existingObject *unstructured.Unstructured, opts ApplyOptions) bool {
	if utils.AnyInMetadata(desiredObject, opts.ExclusionSelector) ||
		(existingObject != nil && utils.AnyInMetadata(existingObject, opts.ExclusionSelector)) {
		return true
	}

	if existingObject != nil &&
		existingObject.GetUID() != "" &&
		utils.AnyInMetadata(desiredObject, opts.IfNotPresentSelector) {
		return true
	}

	return false
}
