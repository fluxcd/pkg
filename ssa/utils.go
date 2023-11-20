/*
Copyright 2021 Stefan Prodan
Copyright 2021 The Flux authors

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
	"encoding/json"
	"io"
	"regexp"
	"strings"

	"github.com/fluxcd/cli-utils/pkg/object"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

const fmtSeparator = "/"

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

// FmtObjMetadata returns the object ID in the format <kind>/<namespace>/<name>.
func FmtObjMetadata(obj object.ObjMetadata) string {
	var builder strings.Builder
	builder.WriteString(obj.GroupKind.Kind + fmtSeparator)
	if obj.Namespace != "" {
		builder.WriteString(obj.Namespace + fmtSeparator)
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

// ReadObject decodes a YAML or JSON document from the given reader into an unstructured Kubernetes API object.
func ReadObject(r io.Reader) (*unstructured.Unstructured, error) {
	reader := yamlutil.NewYAMLOrJSONDecoder(r, 2048)
	obj := &unstructured.Unstructured{}
	err := reader.Decode(obj)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// ReadObjects decodes the YAML or JSON documents from the given reader into unstructured Kubernetes API objects.
// The documents which do not subscribe to the Kubernetes Object interface, are silently dropped from the result.
func ReadObjects(r io.Reader) ([]*unstructured.Unstructured, error) {
	reader := yamlutil.NewYAMLOrJSONDecoder(r, 2048)
	objects := make([]*unstructured.Unstructured, 0)

	for {
		obj := &unstructured.Unstructured{}
		err := reader.Decode(obj)
		if err != nil {
			if err == io.EOF {
				err = nil
				break
			}
			return objects, err
		}

		if obj.IsList() {
			err = obj.EachListItem(func(item runtime.Object) error {
				obj := item.(*unstructured.Unstructured)
				objects = append(objects, obj)
				return nil
			})
			if err != nil {
				return objects, err
			}
			continue
		}

		if IsKubernetesObject(obj) && !IsKustomization(obj) {
			objects = append(objects, obj)
		}
	}

	return objects, nil
}

// ObjectToYAML encodes the given Kubernetes API object to YAML.
func ObjectToYAML(object *unstructured.Unstructured) string {
	var builder strings.Builder
	data, err := yaml.Marshal(object)
	if err != nil {
		return ""
	}
	builder.Write(data)
	builder.WriteString("---\n")

	return builder.String()
}

// ObjectsToYAML encodes the given Kubernetes API objects to a YAML multi-doc.
func ObjectsToYAML(objects []*unstructured.Unstructured) (string, error) {
	var builder strings.Builder
	for _, obj := range objects {
		data, err := yaml.Marshal(obj)
		if err != nil {
			return "", err
		}
		builder.Write(data)
		builder.WriteString("---\n")
	}
	return builder.String(), nil
}

// ObjectsToJSON encodes the given Kubernetes API objects to a YAML multi-doc.
func ObjectsToJSON(objects []*unstructured.Unstructured) (string, error) {
	list := struct {
		ApiVersion string                       `json:"apiVersion,omitempty"`
		Kind       string                       `json:"kind,omitempty"`
		Items      []*unstructured.Unstructured `json:"items,omitempty"`
	}{
		ApiVersion: "v1",
		Kind:       "List",
		Items:      objects,
	}

	data, err := json.MarshalIndent(list, "", "    ")
	if err != nil {
		return "", err
	}

	return string(data), nil
}

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

// Match CEL immutable error variants.
var matchImmutableFieldErrors = []*regexp.Regexp{
	regexp.MustCompile(`.*is\simmutable.*`),
	regexp.MustCompile(`.*immutable\sfield.*`),
}

// IsImmutableError checks if the given error is an immutable error.
func IsImmutableError(err error) bool {
	// Detect immutability like kubectl does
	// https://github.com/kubernetes/kubectl/blob/8165f83007/pkg/cmd/apply/patcher.go#L201
	if errors.IsConflict(err) || errors.IsInvalid(err) {
		return true
	}

	// Detect immutable errors returned by custom admission webhooks and Kubernetes CEL
	// https://kubernetes.io/blog/2022/09/29/enforce-immutability-using-cel/#immutablility-after-first-modification
	for _, fieldError := range matchImmutableFieldErrors {
		if fieldError.MatchString(err.Error()) {
			return true
		}
	}

	return false
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

// SetNativeKindsDefaults sets default values for native Kubernetes objects,
// working around various upstream Kubernetes API bugs.
//
// Deprecated: use NormalizeUnstructuredList or NormalizeUnstructured instead.
func SetNativeKindsDefaults(objects []*unstructured.Unstructured) error {
	return NormalizeUnstructuredList(objects)
}

func containsItemString(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
