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

	"github.com/go-openapi/jsonpointer"
	"golang.org/x/sync/errgroup"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ssaerrors "github.com/fluxcd/pkg/ssa/errors"
	"github.com/fluxcd/pkg/ssa/jsondiff"
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

	// CustomStageKinds defines a set of Kubernetes resource types that should be applied
	// in a separate stage after CRDs and before namespaced objects.
	CustomStageKinds map[schema.GroupKind]struct{} `json:"customStageKinds,omitempty"`

	// MigrateAPIVersion, when enabled, rewrites every managed fields entry
	// on the existing object to match the API version of the applied object
	// before each apply.
	//
	// This is needed after a CRD adds a new version that introduces fields
	// with default values: without migration, any managed fields entry still
	// tagged with the old API version causes the API server to fail the
	// apply with "field not declared in schema" for the new defaulted field.
	MigrateAPIVersion bool `json:"migrateAPIVersion,omitempty"`

	// DriftIgnoreRules defines a list of JSON pointer ignore rules that are used to
	// remove specific fields from objects before applying them.
	// This is useful for ignoring fields that are managed by other controllers
	// (e.g. VPA, HPA) and would otherwise cause drift.
	DriftIgnoreRules []jsondiff.IgnoreRule `json:"driftIgnoreRules,omitempty"`
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

// compiledIgnoreRules is a map of pre-compiled selectors to their associated
// JSON pointer paths, used to avoid recompiling selectors for each object.
type compiledIgnoreRules map[*jsondiff.SelectorRegex][]string

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

	var patched bool
	if opts.MigrateAPIVersion && getError == nil {
		var err error
		patched, err = m.migrateAPIVersion(ctx, existingObject, object.GetAPIVersion())
		if err != nil {
			return nil, fmt.Errorf("%s failed to migrate API version: %w", utils.FmtUnstructured(existingObject), err)
		}
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

	patchedCleanupMetadata, err := m.cleanupMetadata(ctx, object, existingObject, opts.Cleanup)
	if err != nil {
		return nil, fmt.Errorf("%s metadata.managedFields cleanup failed: %w",
			utils.FmtUnstructured(existingObject), err)
	}
	patched = patched || patchedCleanupMetadata

	// Compile ignore rules once for both drift detection and conditional field stripping.
	var compiled compiledIgnoreRules
	if existingObject.GetResourceVersion() != "" && len(opts.DriftIgnoreRules) > 0 {
		compiled, err = compileIgnoreRules(opts.DriftIgnoreRules)
		if err != nil {
			return nil, err
		}
	}

	// Do not apply objects that have not drifted to avoid bumping the resource version.
	// Ignored fields are excluded from the comparison so that differences in fields
	// managed by other controllers (e.g. VPA, HPA) do not trigger unnecessary applies.
	drifted, err := m.hasDriftedWithIgnore(existingObject, dryRunObject, compiled)
	if err != nil {
		return nil, err
	}
	if !patched && !drifted {
		return m.changeSetEntry(object, UnchangedAction), nil
	}

	appliedObject := object.DeepCopy()

	// Strip only the ignored fields that have actually drifted between the
	// existing and dry-run objects. Fields that match are kept in the payload
	// to preserve Flux's ownership.
	if compiled != nil {
		driftedPaths := computeDriftedPaths(existingObject, dryRunObject, compiled)
		if len(driftedPaths) > 0 {
			patch := jsondiff.GenerateRemovePatch(driftedPaths...)
			if err := jsondiff.ApplyPatchToUnstructured(appliedObject, patch); err != nil {
				return nil, err
			}
		}
	}

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
	driftedIgnorePaths := make([]jsondiff.IgnorePaths, len(objects))

	// Compile ignore rules once for drift detection and conditional field stripping.
	var compiled compiledIgnoreRules
	if len(opts.DriftIgnoreRules) > 0 {
		var err error
		compiled, err = compileIgnoreRules(opts.DriftIgnoreRules)
		if err != nil {
			return nil, err
		}
	}

	{
		g, ctx := errgroup.WithContext(ctx)
		g.SetLimit(m.concurrency)
		for i, object := range objects {

			g.Go(func() error {
				utils.RemoveCABundleFromCRD(object)

				existingObject := &unstructured.Unstructured{}
				existingObject.SetGroupVersionKind(object.GroupVersionKind())
				getError := m.client.Get(ctx, client.ObjectKeyFromObject(object), existingObject)

				if m.shouldSkipApply(object, existingObject, opts) {
					changes[i] = *m.changeSetEntry(object, SkippedAction)
					return nil
				}

				var patched bool
				if opts.MigrateAPIVersion && getError == nil {
					var err error
					patched, err = m.migrateAPIVersion(ctx, existingObject, object.GetAPIVersion())
					if err != nil {
						return fmt.Errorf("%s failed to migrate API version: %w", utils.FmtUnstructured(existingObject), err)
					}
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

				patchedCleanupMetadata, err := m.cleanupMetadata(ctx, object, existingObject, opts.Cleanup)
				if err != nil {
					return fmt.Errorf("%s metadata.managedFields cleanup failed: %w",
						utils.FmtUnstructured(existingObject), err)
				}
				patched = patched || patchedCleanupMetadata

				drifted, err := m.hasDriftedWithIgnore(existingObject, dryRunObject, compiled)
				if err != nil {
					return err
				}
				if patched || drifted {
					toApply[i] = object
					// Compute drifted paths while existingObject and dryRunObject are available.
					if compiled != nil && existingObject.GetResourceVersion() != "" {
						driftedIgnorePaths[i] = computeDriftedPaths(existingObject, dryRunObject, compiled)
					}
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

	for i, object := range toApply {
		if object != nil {
			appliedObject := object.DeepCopy()
			if changes[i].Action != CreatedAction && len(driftedIgnorePaths[i]) > 0 {
				patch := jsondiff.GenerateRemovePatch(driftedIgnorePaths[i]...)
				if err := jsondiff.ApplyPatchToUnstructured(appliedObject, patch); err != nil {
					return nil, err
				}
			}
			if err := m.apply(ctx, appliedObject); err != nil {
				return nil, fmt.Errorf("%s apply failed: %w", utils.FmtUnstructured(appliedObject), err)
			}
		}
	}

	changeSet := NewChangeSet()
	changeSet.Append(changes)

	return changeSet, nil
}

// ApplyAllStaged extracts the cluster and class definitions, applies them with ApplyAll,
// waits for them to become ready, then it applies all the other objects.
// This function should be used when the given objects have a mix of custom resource definition
// and custom resources, or a mix of namespace definitions with namespaced objects.
// If an error occurs during the apply of the cluster or class definitions, the change set is
// returned with the applied entries, up to that point, and the error is returned.
func (m *ResourceManager) ApplyAllStaged(ctx context.Context, objects []*unstructured.Unstructured, opts ApplyOptions) (*ChangeSet, error) {
	changeSet := NewChangeSet()

	var (
		// Contains only CRDs, ClusterRoles, and Namespaces.
		defStage []*unstructured.Unstructured

		// Contains only Class definitions.
		classStage []*unstructured.Unstructured

		// Contains the custom kinds.
		customStage []*unstructured.Unstructured

		// Contains all objects except for cluster definitions and class definitions.
		resStage []*unstructured.Unstructured
	)

	for _, o := range objects {
		switch {
		case utils.IsClusterDefinition(o):
			defStage = append(defStage, o)
		case utils.IsClassDefinition(o):
			classStage = append(classStage, o)
		case utils.IsCustomStage(o, opts.CustomStageKinds):
			customStage = append(customStage, o)
		default:
			resStage = append(resStage, o)
		}
	}

	// Apply CRDs, ClusterRoles, and Namespaces first and wait for them to become ready.
	if len(defStage) > 0 {
		cs, err := m.ApplyAll(ctx, defStage, opts)
		if err != nil {
			return changeSet, err
		}
		changeSet.Append(cs.Entries)

		if err := m.WaitForSet(cs.ToObjMetadataSet(), WaitOptions{Interval: opts.WaitInterval, Timeout: opts.WaitTimeout}); err != nil {
			return changeSet, err
		}
	}

	// Apply Class definitions next, if any, and wait for them to become ready.
	if len(classStage) > 0 {
		cs, err := m.ApplyAll(ctx, classStage, opts)
		if err != nil {
			return changeSet, err
		}
		changeSet.Append(cs.Entries)

		if err := m.WaitForSet(cs.ToObjMetadataSet(), WaitOptions{Interval: opts.WaitInterval, Timeout: opts.WaitTimeout}); err != nil {
			return changeSet, err
		}
	}

	// Apply custom staged objects next.
	if len(customStage) > 0 {
		cs, err := m.ApplyAll(ctx, customStage, opts)
		if err != nil {
			return changeSet, err
		}
		changeSet.Append(cs.Entries)
	}

	// Finally, apply all the other resources.
	cs, err := m.ApplyAll(ctx, resStage, opts)
	if err != nil {
		return changeSet, err
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

// migrateAPIVersion rewrites every managed fields entry on existingObject
// to desiredAPIVersion via a JSON patch. See ApplyOptions.MigrateAPIVersion
// for the motivation. Every entry is rewritten, regardless of the field
// manager that owns it or whether it sits on a subresource: any entry
// left at an older API version can make the next apply fail with "field
// not declared in schema". On success existingObject is updated in-place
// with the server's response. Returns whether a patch was actually
// applied (false means there was nothing to migrate).
func (m *ResourceManager) migrateAPIVersion(ctx context.Context,
	existingObject *unstructured.Unstructured,
	desiredAPIVersion string) (bool, error) {

	// Build patch.
	patch, err := PatchMigrateToVersion(existingObject, desiredAPIVersion)
	if err != nil {
		return false, fmt.Errorf("failed to create patch for migrating managed fields API version: %w", err)
	}
	if len(patch) == 0 {
		return false, nil
	}

	// Apply patch.
	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return false, fmt.Errorf("failed to marshal patch for migrating managed fields API version: %w", err)
	}
	rawPatch := client.RawPatch(types.JSONPatchType, patchBytes)
	if err := m.client.Patch(ctx, existingObject, rawPatch, client.FieldOwner(m.owner.Field)); err != nil {
		return false, fmt.Errorf("failed to migrate managed fields API version to %s: %w", desiredAPIVersion, err)
	}

	return true, nil
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
	var patches []JSONPatch

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

// compileIgnoreRules compiles the selectors from the given ignore rules into
// regular expressions. The compiled rules can be reused across multiple objects.
func compileIgnoreRules(rules []jsondiff.IgnoreRule) (compiledIgnoreRules, error) {
	sm := make(compiledIgnoreRules, len(rules))
	for _, rule := range rules {
		sr, err := jsondiff.NewSelectorRegex(rule.Selector)
		if err != nil {
			return nil, fmt.Errorf("failed to create ignore rule selector: %w", err)
		}
		sm[sr] = rule.Paths
	}
	return sm, nil
}

// removeIgnoredFields removes the fields matched by the given pre-compiled
// ignore rules from obj. Selectors are evaluated against matchObj so that
// existing and dry-run copies are stripped based on the same decision.
func removeIgnoredFields(matchObj, obj *unstructured.Unstructured, rules compiledIgnoreRules) error {
	var ignorePaths jsondiff.IgnorePaths
	for sr, paths := range rules {
		if sr.MatchUnstructured(matchObj) {
			ignorePaths = append(ignorePaths, paths...)
		}
	}

	if len(ignorePaths) > 0 {
		patch := jsondiff.GenerateRemovePatch(ignorePaths...)
		if err := jsondiff.ApplyPatchToUnstructured(obj, patch); err != nil {
			return err
		}
	}

	return nil
}

// lookupJSONPointer resolves an RFC 6901 JSON pointer against the unstructured
// object's content. A missing path is reported as (nil, false, nil).
func lookupJSONPointer(obj *unstructured.Unstructured, pointer string) (any, bool, error) {
	ptr, err := jsonpointer.New(pointer)
	if err != nil {
		return nil, false, err
	}
	val, _, err := ptr.Get(obj.Object)
	if err != nil {
		// jsonpointer returns an error when any segment of the pointer cannot
		// be resolved; treat that as "not present" rather than a hard failure.
		return nil, false, nil
	}
	return val, true, nil
}

// computeDriftedPaths returns the subset of ignored paths whose values differ
// between existingObject and dryRunObject.
func computeDriftedPaths(
	existingObject, dryRunObject *unstructured.Unstructured,
	rules compiledIgnoreRules,
) jsondiff.IgnorePaths {
	var drifted jsondiff.IgnorePaths
	for sr, paths := range rules {
		if sr.MatchUnstructured(dryRunObject) {
			for _, path := range paths {
				existingVal, ef, eerr := lookupJSONPointer(existingObject, path)
				dryRunVal, df, derr := lookupJSONPointer(dryRunObject, path)
				if eerr != nil || derr != nil || ef != df ||
					!apiequality.Semantic.DeepEqual(existingVal, dryRunVal) {
					drifted = append(drifted, path)
				}
			}
		}
	}
	return drifted
}

// hasDriftedWithIgnore is like hasDrifted but strips ignored fields from deep
// copies before comparing. If compiled is nil, it falls back to hasDrifted.
// Selector matching is done against dryRunObject so both copies are stripped
// on the same decision, matching computeDriftedPaths.
func (m *ResourceManager) hasDriftedWithIgnore(
	existingObject, dryRunObject *unstructured.Unstructured,
	compiled compiledIgnoreRules,
) (bool, error) {
	if compiled == nil {
		return m.hasDrifted(existingObject, dryRunObject), nil
	}
	existingCopy := existingObject.DeepCopy()
	dryRunCopy := dryRunObject.DeepCopy()
	if err := removeIgnoredFields(dryRunObject, existingCopy, compiled); err != nil {
		return false, err
	}
	if err := removeIgnoredFields(dryRunObject, dryRunCopy, compiled); err != nil {
		return false, err
	}
	return m.hasDrifted(existingCopy, dryRunCopy), nil
}
