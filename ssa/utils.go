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

	"github.com/google/go-cmp/cmp"
	appsv1 "k8s.io/api/apps/v1"
	hpav2beta1 "k8s.io/api/autoscaling/v2beta1"
	hpav2beta2 "k8s.io/api/autoscaling/v2beta2"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

func getNestedMap(object *unstructured.Unstructured) (map[string]interface{}, bool, error) {
	dryRunData, foundDryRun, err := unstructured.NestedMap(object.Object, "data")
	if err != nil {
		return nil, foundDryRun, err
	}

	return dryRunData, foundDryRun, nil
}

func setNestedMap(object *unstructured.Unstructured, data map[string]interface{}) error {
	err := unstructured.SetNestedMap(object.Object, data, "data")
	if err != nil {
		return err
	}

	return nil
}

func cmpMaskData(currentData, futureData map[string]interface{}) (map[string]interface{}, map[string]interface{}) {
	for k, currentVal := range currentData {
		futureVal, ok := futureData[k]
		if !ok {
			// if the key is not in the existing object, we apply the default masking
			currentData[k] = defaultMask
			continue
		}
		// if the key is in the existing object, we need to check if the value is the same
		if cmp.Diff(currentVal, futureVal) != "" {
			// if the value is different, we need to apply different masking
			currentData[k] = defaultMask
			futureData[k] = diffMask
			continue
		}
		// if the value is the same, we apply the same masking
		currentData[k] = defaultMask
		futureData[k] = defaultMask
	}

	for k := range futureData {
		if _, ok := currentData[k]; !ok {
			// if the key is not in the dry run object, we apply the default masking
			futureData[k] = defaultMask
		}
	}

	return currentData, futureData
}

// maskSecret replaces the data key values with the given mask.
func maskSecret(data map[string]interface{}, object *unstructured.Unstructured, mask string) (*unstructured.Unstructured, error) {

	for k := range data {
		data[k] = mask
	}

	err := setNestedMap(object, data)
	if err != nil {
		return nil, err
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

// IsImmutableError checks if the given error is an immutable error.
func IsImmutableError(err error) bool {
	// Detect immutability like kubectl does
	// https://github.com/kubernetes/kubectl/blob/8165f83007/pkg/cmd/apply/patcher.go#L201
	if errors.IsConflict(err) || errors.IsInvalid(err) {
		return true
	}
	return false
}

// AnyInMetadata searches for the specified key-value pairs in labels and annotations,
// returns true if at least one key-value pair matches.
func AnyInMetadata(object *unstructured.Unstructured, metadata map[string]string) bool {
	for key, val := range metadata {
		if object.GetLabels()[key] == val || object.GetAnnotations()[key] == val {
			return true
		}
	}
	return false
}

// SetNativeKindsDefaults implements workarounds for server-side apply upstream bugs affecting Kubernetes < 1.22
// ContainerPort missing default TCP proto: https://github.com/kubernetes-sigs/structured-merge-diff/issues/130
// ServicePort missing default TCP proto: https://github.com/kubernetes/kubernetes/pull/98576
// PodSpec resources missing int to string conversion for e.g. 'cpu: 2'
// secret.stringData key replacement add an extra key in the resulting data map: https://github.com/kubernetes/kubernetes/issues/108008
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
		switch u.GetAPIVersion() {
		case "v1":
			switch u.GetKind() {
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
			case "Secret":
				var s corev1.Secret
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &s)
				if err != nil {
					return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
				}
				convertStringDataToData(&s)
				out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&s)
				if err != nil {
					return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
				}
				u.Object = out
			}

		case "apps/v1":
			switch u.GetKind() {
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
			}
		}

		switch u.GetKind() {
		case "HorizontalPodAutoscaler":
			switch u.GetAPIVersion() {
			case "autoscaling/v2beta1":
				var d hpav2beta1.HorizontalPodAutoscaler
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &d)
				if err != nil {
					return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
				}
				out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
				if err != nil {
					return fmt.Errorf("%s validation error: %w", FmtUnstructured(u), err)
				}
				u.Object = out
			case "autoscaling/v2beta2":
				var d hpav2beta2.HorizontalPodAutoscaler
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

		// remove fields that are not supposed to be present in manifests
		unstructured.RemoveNestedField(u.Object, "metadata", "creationTimestamp")

		// remove status but for CRDs (kstatus wait doesn't work with empty status fields)
		if u.GetKind() != "CustomResourceDefinition" {
			unstructured.RemoveNestedField(u.Object, "status")
		}

	}
	return nil
}

// Fix bug in server-side dry-run apply that duplicates the first item in the metrics array
// and inserts an empty metric as the last item in the array.
func fixHorizontalPodAutoscaler(object *unstructured.Unstructured) error {
	if object.GetKind() == "HorizontalPodAutoscaler" {
		switch object.GetAPIVersion() {
		case "autoscaling/v2beta2":
			var d hpav2beta2.HorizontalPodAutoscaler
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, &d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(object), err)
			}

			var metrics []hpav2beta2.MetricSpec
			for _, metric := range d.Spec.Metrics {
				found := false
				for _, existing := range metrics {
					if apiequality.Semantic.DeepEqual(metric, existing) {
						found = true
						break
					}
				}
				if !found && metric.Type != "" {
					metrics = append(metrics, metric)
				}
			}

			d.Spec.Metrics = metrics

			out, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&d)
			if err != nil {
				return fmt.Errorf("%s validation error: %w", FmtUnstructured(object), err)
			}
			object.Object = out
		}
	}
	return nil
}

func containsItemString(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func convertStringDataToData(secret *corev1.Secret) {
	// StringData overwrites Data
	if len(secret.StringData) > 0 {
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		for k, v := range secret.StringData {
			secret.Data[k] = []byte(v)
		}

		secret.StringData = nil
	}
}
