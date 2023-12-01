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

	"github.com/fluxcd/cli-utils/pkg/object"
)

const separator = "/"

// FmtObjMetadata returns the object ID in the format <kind>/<namespace>/<name>.
func FmtObjMetadata(obj object.ObjMetadata) string {
	var builder strings.Builder
	builder.WriteString(obj.GroupKind.Kind + separator)
	if obj.Namespace != "" {
		builder.WriteString(obj.Namespace + separator)
	}
	builder.WriteString(obj.Name)
	return builder.String()
}

// FmtUnstructured returns the object ID in the format <kind>/<namespace>/<name>.
func FmtUnstructured(obj *unstructured.Unstructured) string {
	return FmtObjMetadata(object.UnstructuredToObjMetadata(obj))
}

// FmtUnstructuredList returns a line per object in the format <kind>/<namespace>/<name>.
func FmtUnstructuredList(objects []*unstructured.Unstructured) string {
	var b strings.Builder
	for _, obj := range objects {
		b.WriteString(FmtObjMetadata(object.UnstructuredToObjMetadata(obj)) + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}
