/*
Copyright 2023 The Flux authors

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

package jsondiff

import (
	"context"
	"fmt"

	"github.com/wI2L/jsondiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/ssa"
)

// IgnorePathSelector contains the information needed to ignore certain paths
// in a (set of) Kubernetes resource(s).
type IgnorePathSelector struct {
	// Paths is a list of JSON pointers to ignore.
	Paths []string
	// Selector is a selector that matches the resources to ignore.
	Selector *Selector
}

// UnstructuredList performs a server-side apply dry-run and returns a DiffSet
// containing the changes detected. It takes a list of Kubernetes resources
// and a list of options. The options can be used to ignore certain paths in
// certain resources, or to ignore certain resources altogether.
func UnstructuredList(ctx context.Context, c client.Client, objs []*unstructured.Unstructured, opts ...ListOption) (DiffSet, error) {
	o := &ListOptions{}
	o.ApplyOptions(opts)

	var sm = make(map[*SelectorRegex][]string, len(o.IgnorePathSelectors))
	for _, ips := range o.IgnorePathSelectors {
		sr, err := NewSelectorRegex(ips.Selector)
		if err != nil {
			return nil, fmt.Errorf("failed to create selector regex: %w", err)
		}
		sm[sr] = ips.Paths
	}

	var resOpts []ResourceOption
	for _, ro := range opts {
		if r, ok := ro.(ResourceOption); ok {
			resOpts = append(resOpts, r)
		}
	}

	var set DiffSet
	for _, obj := range objs {
		obj := obj

		if ssa.AnyInMetadata(obj, o.ExclusionSelectors) {
			set = append(set, NewDiffForUnstructured(obj, DiffTypeExclude, nil))
			continue
		}

		var ignorePaths IgnorePaths
		for sr, paths := range sm {
			if sr.MatchUnstructured(obj) {
				ignorePaths = append(ignorePaths, paths...)
			}
		}

		diff, err := Unstructured(ctx, c, obj, append(resOpts, ignorePaths)...)
		if err != nil {
			return nil, err
		}
		set = append(set, diff)
	}
	return set, nil
}

// Unstructured performs a server-side apply dry-run and returns the type of change
// detected, and a JSON patch with the changes. If the resource does not exist,
// it returns DiffTypeCreate. If the resource exists and is identical to the
// dry-run object, it returns DiffTypeNone. Otherwise, it returns
// DiffTypeUpdate and a JSON patch with the changes.
func Unstructured(ctx context.Context, c client.Client, obj *unstructured.Unstructured, opts ...ResourceOption) (*Diff, error) {
	o := &ResourceOptions{}
	o.ApplyOptions(opts)

	existingObj := obj.DeepCopy()
	if err := c.Get(ctx, client.ObjectKeyFromObject(obj), existingObj); client.IgnoreNotFound(err) != nil {
		return nil, err
	}

	dryRunObj := obj.DeepCopy()
	patchOpts := []client.PatchOption{
		client.DryRunAll,
		client.ForceOwnership,
		client.FieldOwner(o.FieldManager),
	}
	if err := c.Patch(ctx, dryRunObj, client.Apply, patchOpts...); err != nil {
		return nil, ssa.NewDryRunErr(err, obj)
	}

	if dryRunObj.GetResourceVersion() == "" {
		return NewDiffForUnstructured(obj, DiffTypeCreate, nil), nil
	}

	if err := ssa.NormalizeDryRunUnstructured(dryRunObj); err != nil {
		return nil, err
	}

	// Remove any ignored JSON pointers from the dry-run and existing objects.
	if len(o.IgnorePaths) > 0 {
		patch := GenerateRemovePatch(o.IgnorePaths...)
		if err := ApplyPatchToUnstructured(dryRunObj, patch); err != nil {
			return nil, err
		}
		if err := ApplyPatchToUnstructured(existingObj, patch); err != nil {
			return nil, err
		}
	}

	var diffOpts []jsondiff.Option
	if o.Rationalize {
		diffOpts = append(diffOpts, jsondiff.Rationalize())
	}

	// Calculate the JSON patch between the dry-run and existing objects.
	var patch jsondiff.Patch
	metaPatch, err := diffUnstructuredMetadata(existingObj, dryRunObj, diffOpts...)
	if err != nil {
		return nil, err
	}
	patch = append(patch, metaPatch...)

	resPatch, err := diffUnstructured(existingObj, dryRunObj, diffOpts...)
	if err != nil {
		return nil, err
	}
	patch = append(patch, resPatch...)

	if len(patch) == 0 {
		return NewDiffForUnstructured(obj, DiffTypeNone, nil), nil
	}

	// Mask secrets if requested.
	if o.MaskSecrets {
		if gvk := obj.GroupVersionKind(); gvk.Group == "" && gvk.Kind == "Secret" {
			patch = MaskSecretPatchData(patch)
		}
	}
	return NewDiffForUnstructured(obj, DiffTypeUpdate, patch), nil
}

// diffUnstructuredMetadata returns a JSON patch with the differences between
// the labels and annotations metadata of the given objects. It ignores other
// fields, and only returns "replace" and "add" changes.
func diffUnstructuredMetadata(x, y *unstructured.Unstructured, opts ...jsondiff.Option) (jsondiff.Patch, error) {
	xMeta, yMeta := copyAnnotationsAndLabels(x), copyAnnotationsAndLabels(y)
	patch, err := jsondiff.Compare(xMeta, yMeta, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to compare annotations and labels of objects: %w", err)
	}

	var filteredPatch jsondiff.Patch
	for _, change := range patch {
		switch change.Type {
		case jsondiff.OperationReplace, jsondiff.OperationAdd:
			filteredPatch = append(filteredPatch, change)
		default:
			// Ignore other changes (like "remove") to avoid false positives due
			// to core Kubernetes controllers adding labels to resources.
		}
	}

	return filteredPatch, nil
}

// diffUnstructured returns a JSON patch with the differences between the given
// objects while ignoring "metadata" and "status" fields.
func diffUnstructured(x, y *unstructured.Unstructured, opts ...jsondiff.Option) (jsondiff.Patch, error) {
	xSpec, ySpec := removeMetadataAndStatus(x), removeMetadataAndStatus(y)
	patch, err := jsondiff.Compare(xSpec.Object, ySpec.Object, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to compare objects: %w", err)
	}
	return patch, nil
}

// copyAnnotationsAndLabels returns a copy of the given object with only the
// metadata annotations and labels fields set.
func copyAnnotationsAndLabels(obj *unstructured.Unstructured) *unstructured.Unstructured {
	c := &unstructured.Unstructured{
		Object: make(map[string]interface{}),
	}

	annotations, ok, _ := unstructured.NestedFieldCopy(obj.Object, "metadata", "annotations")
	if ok {
		_ = unstructured.SetNestedField(c.Object, annotations, "metadata", "annotations")
	}

	labels, ok, _ := unstructured.NestedFieldCopy(obj.Object, "metadata", "labels")
	if ok {
		_ = unstructured.SetNestedField(c.Object, labels, "metadata", "labels")
	}

	return c
}

// removeMetadataAndStatus returns a copy of the given object with the metadata
// and status fields removed.
func removeMetadataAndStatus(obj *unstructured.Unstructured) *unstructured.Unstructured {
	c := obj.DeepCopy()
	unstructured.RemoveNestedField(c.Object, "metadata")
	unstructured.RemoveNestedField(c.Object, "status")
	return c
}
