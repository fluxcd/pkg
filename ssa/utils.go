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
	"fmt"
	"io"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/cli-utils/pkg/object"
	"sigs.k8s.io/yaml"
)

const fmtSeparator = "/"

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
	return FmtObjMetadata(object.UnstructuredToObjMeta(obj))
}

// FmtUnstructuredList returns a line per object in the format <kind>/<namespace>/<name>.
func FmtUnstructuredList(objects []*unstructured.Unstructured) string {
	var b strings.Builder
	for _, obj := range objects {
		b.WriteString(FmtObjMetadata(object.UnstructuredToObjMeta(obj)) + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// MaskSecret replaces the data key values with the given mask.
func MaskSecret(object *unstructured.Unstructured, mask string) (*unstructured.Unstructured, error) {
	data, found, err := unstructured.NestedMap(object.Object, "data")
	if err != nil {
		return nil, err
	}

	if found {
		for k, _ := range data {
			data[k] = mask
		}

		err = unstructured.SetNestedMap(object.Object, data, "data")
		if err != nil {
			return nil, err
		}
	}

	return object, err
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
			err = obj.EachListItem(func(item apiruntime.Object) error {
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
		Kind:       "ListMeta",
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
	kind := object.GetKind()
	switch strings.ToLower(kind) {
	case "customresourcedefinition":
		return true
	case "namespace":
		return true
	}
	return false
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
	if object.GetKind() == "Kustomization" && object.GroupVersionKind().GroupKind().Group == "kustomize.config.k8s.io" {
		return true
	}
	return false
}

func isImmutableError(err error) bool {
	for _, s := range []string{"field is immutable", "cannot change roleRef"} {
		if strings.Contains(err.Error(), s) {
			return true
		}
	}
	return false
}

// SetNativeKindsDefaults implements workarounds for server-side apply upstream bugs affecting Kubernetes < 1.22
// ContainerPort missing default TCP proto: https://github.com/kubernetes-sigs/structured-merge-diff/issues/130
// ServicePort missing default TCP proto: https://github.com/kubernetes/kubernetes/pull/98576
// PodSpec resources missing int to string conversion for e.g. 'cpu: 2'
func SetNativeKindsDefaults(objects []*unstructured.Unstructured) error {

	var setProtoDefault = func(spec *corev1.PodSpec) {
		for _, c := range spec.Containers {
			for i, port := range c.Ports {
				if port.Protocol == "" {
					c.Ports[i].Protocol = "TCP"
				}
			}
		}
	}

	for _, u := range objects {
		switch u.GetKind() {
		case "Pod":
			var d corev1.Pod
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}

			setProtoDefault(&d.Spec)

			out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}
			u.Object = out
		case "Deployment":
			var d appsv1.Deployment
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}

			setProtoDefault(&d.Spec.Template.Spec)

			out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}
			u.Object = out
		case "StatefulSet":
			var d appsv1.StatefulSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}

			setProtoDefault(&d.Spec.Template.Spec)

			out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}
			u.Object = out
		case "DaemonSet":
			var d appsv1.DaemonSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}

			setProtoDefault(&d.Spec.Template.Spec)

			out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}
			u.Object = out
		case "ReplicaSet":
			var d appsv1.ReplicaSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}

			setProtoDefault(&d.Spec.Template.Spec)

			out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}
			u.Object = out
		case "Service":
			var d corev1.Service
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}

			// set port protocol default
			// workaround for: https://github.com/kubernetes-sigs/structured-merge-diff/issues/130
			for i, port := range d.Spec.Ports {
				if port.Protocol == "" {
					d.Spec.Ports[i].Protocol = "TCP"
				}
			}

			out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
			}
			u.Object = out
		case "HorizontalPodAutoscaler":
			if strings.Contains(u.GetAPIVersion(), "autoscaling/v2") {
				var d autoscalingv2.HorizontalPodAutoscaler
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &d)
				if err != nil {
					return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
				}
				out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
				if err != nil {
					return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
				}
				u.Object = out
			}
		}
	}
	return nil
}
