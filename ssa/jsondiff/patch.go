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
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/wI2L/jsondiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// GenerateRemovePatch generates a JSON patch that removes the given JSON
// pointer paths.
func GenerateRemovePatch(paths ...string) jsondiff.Patch {
	var patch jsondiff.Patch
	for _, p := range paths {
		patch = append(patch, jsondiff.Operation{
			Type: jsondiff.OperationRemove,
			Path: p,
		})
	}
	return patch
}

// ApplyPatchToUnstructured applies the given JSON patch to the given
// unstructured object. The patch is applied in-place.
// It permits the patch to contain "remove" operations that target non-existing
// paths.
func ApplyPatchToUnstructured(obj *unstructured.Unstructured, patch jsondiff.Patch) error {
	if len(patch) == 0 {
		return nil
	}

	uJSON, err := obj.MarshalJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal unstructured object: %w", err)
	}

	// Slightly awkward conversion from jsondiff.Patch to jsonpatch.Patch.
	// This is necessary because the jsondiff library does not support applying
	// patches, while the jsonpatch library does not support generating patches.
	// To not expose this discrepancy to the user, we convert the jsondiff.Patch
	// into a jsonpatch.Patch, and then apply it.

	patchJSON, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON patch: %w", err)
	}

	var patchApplier jsonpatch.Patch
	if err = json.Unmarshal(patchJSON, &patchApplier); err != nil {
		return fmt.Errorf("failed to transform jsondiff.Patch into jsonpatch.Patch: %w", err)
	}

	if uJSON, err = patchApplier.ApplyWithOptions(uJSON, &jsonpatch.ApplyOptions{
		SupportNegativeIndices:   true,
		AccumulatedCopySizeLimit: 0,
		AllowMissingPathOnRemove: true,
	}); err != nil {
		return err
	}

	if err := obj.UnmarshalJSON(uJSON); err != nil {
		return fmt.Errorf("failed to unmarshal patched JSON into unstructured object: %w", err)
	}
	return nil
}
