/*
Copyright 2022 Stefan Prodan
Copyright 2022 The Flux authors

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
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// jsonPatch defines a patch as specified by RFC 6902
// https://www.rfc-editor.org/rfc/rfc6902
type jsonPatch struct {
	Operation string `json:"op"`
	Path      string `json:"path"`
	Value     string `json:"value,omitempty"`
}

// newPatchRemove returns a jsonPatch for removing the specified path
func newPatchRemove(path string) jsonPatch {
	return jsonPatch{
		Operation: "remove",
		Path:      path,
	}
}

// patchRemoveAnnotations returns a jsonPatch array for removing annotations with matching keys
func patchRemoveAnnotations(object *unstructured.Unstructured, keys []string) []jsonPatch {
	var patches []jsonPatch
	annotations := object.GetAnnotations()
	for _, key := range keys {
		if _, ok := annotations[key]; ok {
			path := fmt.Sprintf("/metadata/annotations/%s", strings.ReplaceAll(key, "/", "~1"))
			patches = append(patches, newPatchRemove(path))
		}
	}
	return patches
}

// patchRemoveFieldsManagers returns a jsonPatch array for removing managers with matching prefix and operation type
func patchRemoveFieldsManagers(object *unstructured.Unstructured, managers []string, operation metav1.ManagedFieldsOperationType) []jsonPatch {
	var patches []jsonPatch
	entries := object.GetManagedFields()
	for entryIndex, entry := range entries {
		if entry.Operation == operation {
			for _, manager := range managers {
				if strings.HasPrefix(entry.Manager, manager) {
					path := fmt.Sprintf("/metadata/managedFields/%v", entryIndex)
					patches = append(patches, newPatchRemove(path))
					break
				}
			}
		}
	}
	return patches
}
