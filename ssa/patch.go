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
	"bytes"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

const managedFieldsPath = "/metadata/managedFields"

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

// FieldManager identifies a workflow that's managing fields.
type FieldManager struct {
	// Name is the name of the workflow managing fields.
	Name string `json:"name"`

	// OperationType is the type of operation performed by this manager, can be 'update' or 'apply'.
	OperationType metav1.ManagedFieldsOperationType `json:"operationType"`
}

// patchRemoveFieldsManagers returns a jsonPatch array for removing managers with matching prefix and operation type.
func patchRemoveFieldsManagers(object *unstructured.Unstructured, managers []FieldManager) []jsonPatch {
	objEntries := object.GetManagedFields()

	var patches []jsonPatch
	entries := make([]metav1.ManagedFieldsEntry, 0, len(objEntries))
	for _, entry := range objEntries {
		exclude := false
		for _, manager := range managers {
			if strings.HasPrefix(entry.Manager, manager.Name) &&
				entry.Operation == manager.OperationType &&
				entry.Subresource == "" {
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

	return append(patches, newPatchReplace(managedFieldsPath, entries))
}

// patchReplaceFieldsManagers returns a jsonPatch array for replacing the managers with matching prefix and operation type
// with the specified manager name and an apply operation.
func patchReplaceFieldsManagers(object *unstructured.Unstructured, managers []FieldManager, name string) ([]jsonPatch, error) {
	objEntries := object.GetManagedFields()

	var prevManagedFields metav1.ManagedFieldsEntry
	empty := metav1.ManagedFieldsEntry{}

	// save the previous manager fields
	for _, entry := range objEntries {
		if entry.Manager == name && entry.Operation == metav1.ManagedFieldsOperationApply {
			prevManagedFields = entry
		}
	}

	var patches []jsonPatch
	entries := make([]metav1.ManagedFieldsEntry, 0, len(objEntries))
	edited := false

each_entry:
	for _, entry := range objEntries {
		// no need to append entry for previous managedField
		// since it will be merged with other managedFields and
		// appended at the end.
		if entry == prevManagedFields {
			continue
		}

		for _, manager := range managers {
			if strings.HasPrefix(entry.Manager, manager.Name) &&
				entry.Operation == manager.OperationType &&
				entry.Subresource == "" {

				// if no previous managedField was found,
				// rename the first match.
				if prevManagedFields == empty || prevManagedFields.FieldsV1 == nil {
					entry.Manager = name
					entry.Operation = metav1.ManagedFieldsOperationApply
					prevManagedFields = entry
					edited = true
					continue each_entry
				}

				prevManagedSet, err := FieldsToSet(*prevManagedFields.FieldsV1)
				if err != nil {
					return nil, fmt.Errorf("unable to convert managed field to set: %s", err)
				}
				curManagedSet, err := FieldsToSet(*entry.FieldsV1)
				if err != nil {
					return nil, fmt.Errorf("unable to convert managed field to set: %s", err)
				}

				unionSet := prevManagedSet.Union(&curManagedSet)
				unionField, err := SetToFields(*unionSet)
				if err != nil {
					return nil, fmt.Errorf("unable to convert managed set to field: %s", err)
				}

				prevManagedFields.FieldsV1 = &unionField
				edited = true
				continue each_entry
			}
		}
		entries = append(entries, entry)
	}

	if !edited {
		return nil, nil
	}

	entries = append(entries, prevManagedFields)
	return append(patches, newPatchReplace(managedFieldsPath, entries)), nil
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

// FieldsToSet and SetsToFields are copied from
// https://github.com/kubernetes/apiserver/blob/c4c20f4f7d4ca609906621943c748bc16797a5f3/pkg/endpoints/handlers/fieldmanager/internal/fields.go
// since it is an internal module and can't be imported

// FieldsToSet creates a set paths from an input trie of fields
func FieldsToSet(f metav1.FieldsV1) (s fieldpath.Set, err error) {
	err = s.FromJSON(bytes.NewReader(f.Raw))
	return s, err
}

// SetToFields creates a trie of fields from an input set of paths
func SetToFields(s fieldpath.Set) (f metav1.FieldsV1, err error) {
	f.Raw, err = s.ToJSON()
	return f, err
}
