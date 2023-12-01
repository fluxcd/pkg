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

// IsClusterDefinition checks if the given object is a Kubernetes namespace or a custom resource definition.
func IsClusterDefinition(object *unstructured.Unstructured) bool {
	switch {
	case IsCRD(object):
		return true
	case IsNamespace(object):
		return true
	default:
		return false
	}
}

// IsCRD returns true if the given object is a CustomResourceDefinition.
func IsCRD(object *unstructured.Unstructured) bool {
	return strings.ToLower(object.GetKind()) == "customresourcedefinition" &&
		strings.HasPrefix(object.GetAPIVersion(), "apiextensions.k8s.io/")
}

// IsNamespace returns true if the given object is a Namespace.
func IsNamespace(object *unstructured.Unstructured) bool {
	return strings.ToLower(object.GetKind()) == "namespace" && object.GetAPIVersion() == "v1"
}

// IsKubernetesObject checks if the given object has the minimum required fields to be a Kubernetes object.
func IsKubernetesObject(object *unstructured.Unstructured) bool {
	if object.GetName() == "" || object.GetKind() == "" || object.GetAPIVersion() == "" {
		return false
	}
	return true
}

// IsKustomization checks if the given object is a Kustomize config.
func IsKustomization(object *unstructured.Unstructured) bool {
	return strings.ToLower(object.GetKind()) == "kustomization" &&
		strings.HasPrefix(object.GetAPIVersion(), "kustomize.config.k8s.io/")
}

// IsSecret returns true if the given object is a Kubernetes Secret.
func IsSecret(object *unstructured.Unstructured) bool {
	return strings.ToLower(object.GetKind()) == "secret" && object.GetAPIVersion() == "v1"
}
