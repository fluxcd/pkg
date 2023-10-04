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
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var defaultScheme = scheme.Scheme

// FromUnstructured converts an Unstructured object into a typed Kubernetes
// resource. It only works for API types registered with the default client-go
// scheme.
func FromUnstructured(object *unstructured.Unstructured) (metav1.Object, error) {
	return FromUnstructuredWithScheme(object, defaultScheme)
}

// FromUnstructuredWithScheme converts an Unstructured object into a typed
// Kubernetes resource. It only works for API types registered with the given
// scheme.
func FromUnstructuredWithScheme(object *unstructured.Unstructured, scheme *runtime.Scheme) (metav1.Object, error) {
	typedObject, err := scheme.New(object.GroupVersionKind())
	if err != nil {
		return nil, err
	}

	metaObject, ok := typedObject.(metav1.Object)
	if !ok {
		return nil, err
	}

	if err = runtime.DefaultUnstructuredConverter.FromUnstructured(object.Object, metaObject); err != nil {
		return nil, err
	}
	return metaObject, nil
}

// ToUnstructured converts a typed Kubernetes resource into the Unstructured
// equivalent.
func ToUnstructured(object metav1.Object) (*unstructured.Unstructured, error) {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: u}, nil
}

// NormalizeUnstructured normalizes an Unstructured object by converting it to
// a typed Kubernetes resource, normalizing it, and then converting it back to
// an Unstructured object. It only works for API types registered with the
// default client-go scheme. If the conversion fails, only certain standard
// fields are removed.
func NormalizeUnstructured(object *unstructured.Unstructured) error {
	return NormalizeUnstructuredWithScheme(object, defaultScheme)
}

// NormalizeUnstructuredWithScheme normalizes an Unstructured object by
// converting it to a typed Kubernetes resource, normalizing it, and then
// converting it back to an Unstructured object. It only works for API types
// registered with the given scheme. If the conversion fails, only certain
// standard fields are removed.
func NormalizeUnstructuredWithScheme(object *unstructured.Unstructured, scheme *runtime.Scheme) error {
	if typedObject, err := FromUnstructuredWithScheme(object, scheme); err == nil {
		switch o := typedObject.(type) {
		case *corev1.Pod:
			normalizePodProtoDefault(&o.Spec)
		case *appsv1.Deployment:
			normalizePodProtoDefault(&o.Spec.Template.Spec)
		case *appsv1.StatefulSet:
			normalizePodProtoDefault(&o.Spec.Template.Spec)
		case *appsv1.DaemonSet:
			normalizePodProtoDefault(&o.Spec.Template.Spec)
		case *appsv1.ReplicaSet:
			normalizePodProtoDefault(&o.Spec.Template.Spec)
		case *batchv1.Job:
			normalizePodProtoDefault(&o.Spec.Template.Spec)
		case *batchv1.CronJob:
			normalizePodProtoDefault(&o.Spec.JobTemplate.Spec.Template.Spec)
		case *corev1.Service:
			normalizeServiceProtoDefault(&o.Spec)
		case *corev1.Secret:
			normalizeSecret(o)
		}

		normalizedObject, err := ToUnstructured(typedObject)
		if err != nil {
			return err
		}
		object.Object = normalizedObject.Object
	}

	// Ensure the object has an empty creation timestamp, to avoid
	// issues with the Kubernetes API server rejecting the object
	// or causing any spurious diffs.
	unstructured.SetNestedField(object.Object, nil, "metadata", "creationTimestamp")

	// To ensure kstatus continues to work with CRDs, we need to keep the
	// status field for CRDs.
	if !IsCRD(object) {
		unstructured.RemoveNestedField(object.Object, "status")
	}

	return nil
}

// normalizeServiceProtoDefault sets the default protocol for ports in a
// ServiceSpec.
// xref: https://github.com/kubernetes/kubernetes/pull/98576
func normalizeServiceProtoDefault(spec *corev1.ServiceSpec) {
	for i, port := range spec.Ports {
		if len(port.Protocol) == 0 {
			spec.Ports[i].Protocol = corev1.ProtocolTCP
		}
	}
}

// normalizePodProtoDefault sets the default protocol for ports in a PodSpec.
// xref: https://github.com/kubernetes-sigs/structured-merge-diff/issues/130
func normalizePodProtoDefault(spec *corev1.PodSpec) {
	for _, c := range spec.Containers {
		for i, port := range c.Ports {
			if len(port.Protocol) == 0 {
				c.Ports[i].Protocol = corev1.ProtocolTCP
			}
		}
	}
}

// normalizeSecret converts a Secret's StringData field to Data.
// xref: https://github.com/kubernetes/kubernetes/issues/108008
func normalizeSecret(object *corev1.Secret) {
	if len(object.StringData) > 0 {
		if object.Data == nil {
			object.Data = make(map[string][]byte, len(object.StringData))
		}
		for k, v := range object.StringData {
			object.Data[k] = []byte(v)
		}
		object.StringData = nil
	}
}
