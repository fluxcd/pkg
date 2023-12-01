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

package utils

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// SetCommonMetadata adds the specified labels and annotations to all objects.
// Existing keys will have their values overridden.
func SetCommonMetadata(objects []*unstructured.Unstructured, labels map[string]string, annotations map[string]string) {
	for _, object := range objects {
		lbs := object.GetLabels()
		if lbs == nil {
			lbs = make(map[string]string)
		}

		for k, v := range labels {
			lbs[k] = v
		}

		if len(lbs) > 0 {
			object.SetLabels(lbs)
		}

		ans := object.GetAnnotations()
		if ans == nil {
			ans = make(map[string]string)
		}

		for k, v := range annotations {
			ans[k] = v
		}

		if len(ans) > 0 {
			object.SetAnnotations(ans)
		}
	}
}

// AnyInMetadata searches for the specified key-value pairs in labels and annotations,
// returns true if at least one key-value pair matches.
func AnyInMetadata(object *unstructured.Unstructured, metadata map[string]string) bool {
	labels := object.GetLabels()
	annotations := object.GetAnnotations()
	for key, val := range metadata {
		if (labels[key] != "" && strings.EqualFold(labels[key], val)) ||
			(annotations[key] != "" && strings.EqualFold(annotations[key], val)) {
			return true
		}
	}
	return false
}
