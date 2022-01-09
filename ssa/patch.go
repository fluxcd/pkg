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
	Operation string                      `json:"op"`
	Path      string                      `json:"path"`
	Value     []metav1.ManagedFieldsEntry `json:"value,omitempty"`
}

// newPatchRemove returns a jsonPatch for removing the specified path.
func newPatchRemove(path string) jsonPatch {
	return jsonPatch{
		Operation: "remove",
		Path:      path,
	}
}

// newPatchRemove returns a jsonPatch for removing the specified path.
func newPatchReplace(path string, value []metav1.ManagedFieldsEntry) jsonPatch {
	return jsonPatch{
		Operation: "replace",
		Path:      path,
		Value:     value,
	}
}

// FiledManager identifies a workflow that's managing fields.
type FiledManager struct {
	// Name is the name of the workflow managing fields.
	Name string `json:"name"`

	// OperationType is the type of operation performed by this manager, can be 'update' or 'apply'.
	OperationType metav1.ManagedFieldsOperationType `json:"operationType"`
}

// patchRemoveFieldsManagers returns a jsonPatch array for removing managers with matching prefix and operation type.
func patchRemoveFieldsManagers(object *unstructured.Unstructured, managers []FiledManager) []jsonPatch {
	objEntries := object.GetManagedFields()

	var patches []jsonPatch
	entries := make([]metav1.ManagedFieldsEntry, 0, len(objEntries))
	for _, entry := range objEntries {
		exclude := false
		for _, manager := range managers {
			if strings.HasPrefix(entry.Manager, manager.Name) && entry.Operation == manager.OperationType {
				exclude = true
				break
			}
		}
		if !exclude {
			entries = append(entries, entry)
		}
	}

	if len(entries) == len(objEntries) {
		return nil
	}

	if len(entries) == 0 {
		entries = append(entries, metav1.ManagedFieldsEntry{})
	}

	return append(patches, newPatchReplace("/metadata/managedFields", entries))
}

// patchRemoveAnnotations returns a jsonPatch array for removing annotations with matching keys.
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

// patchRemoveLabels returns a jsonPatch array for removing labels with matching keys.
func patchRemoveLabels(object *unstructured.Unstructured, keys []string) []jsonPatch {
	var patches []jsonPatch
	labels := object.GetLabels()
	for _, key := range keys {
		if _, ok := labels[key]; ok {
			path := fmt.Sprintf("/metadata/labels/%s", strings.ReplaceAll(key, "/", "~1"))
			patches = append(patches, newPatchRemove(path))
		}
	}
	return patches
}
