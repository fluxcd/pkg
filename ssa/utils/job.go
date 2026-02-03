/*
Copyright 2026 The Flux authors

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

	"github.com/fluxcd/cli-utils/pkg/object"
)

// ExtractJobsWithTTL returns the ObjMetadataSet of Jobs that have spec.ttlSecondsAfterFinished set.
func ExtractJobsWithTTL(objects []*unstructured.Unstructured) object.ObjMetadataSet {
	var result object.ObjMetadataSet
	for _, obj := range objects {
		if !IsJob(obj) {
			continue
		}
		// Check if ttlSecondsAfterFinished is set (any value including 0)
		_, found, err := unstructured.NestedInt64(obj.Object, "spec", "ttlSecondsAfterFinished")
		if err == nil && found {
			result = append(result, object.UnstructuredToObjMetadata(obj))
		}
	}
	return result
}

// IsJob returns true if the object is a Kubernetes Job.
func IsJob(obj *unstructured.Unstructured) bool {
	return obj.GetKind() == "Job" &&
		strings.HasPrefix(obj.GetAPIVersion(), "batch/")
}
