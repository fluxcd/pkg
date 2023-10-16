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

package ssa

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	sanitizeMaskDefault = "***"
	sanitizeMaskBefore  = "*** (before)"
	sanitizeMaskAfter   = "*** (after)"
)

// SanitizeUnstructuredData masks the "data" values of an Unstructured object,
// the objects are modified in place.
// If the data value is the same in both objects, the mask is replaced with a
// default mask. If the data value is different, the mask is replaced with a
// before and after mask.
func SanitizeUnstructuredData(old, new *unstructured.Unstructured) error {
	oldData, err := getUnstructuredData(old)
	if err != nil {
		return fmt.Errorf("unable to get data from old object: %w", err)
	}

	newData, err := getUnstructuredData(new)
	if err != nil {
		return fmt.Errorf("unable to get data from new object: %w", err)
	}

	for k := range oldData {
		if _, ok := newData[k]; ok {
			if oldData[k] != newData[k] {
				oldData[k] = sanitizeMaskBefore
				newData[k] = sanitizeMaskAfter
				continue
			}
			newData[k] = sanitizeMaskDefault
		}
		oldData[k] = sanitizeMaskDefault
	}
	for k := range newData {
		if _, ok := oldData[k]; !ok {
			newData[k] = sanitizeMaskDefault
		}
	}

	if old != nil && oldData != nil {
		if err = unstructured.SetNestedMap(old.Object, oldData, "data"); err != nil {
			return fmt.Errorf("masking data in old object failed: %w", err)
		}
	}

	if new != nil && newData != nil {
		if err = unstructured.SetNestedMap(new.Object, newData, "data"); err != nil {
			return fmt.Errorf("masking data in new object failed: %w", err)
		}
	}
	return nil
}

// getUnstructuredData returns the data map from an Unstructured. If the given
// object is nil or does not contain a data map, nil is returned.
func getUnstructuredData(u *unstructured.Unstructured) (map[string]interface{}, error) {
	if u == nil {
		return nil, nil
	}
	data, found, err := unstructured.NestedMap(u.UnstructuredContent(), "data")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	return data, nil
}
