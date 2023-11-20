/*
Copyright 2021 Stefan Prodan.
Copyright 2020 The Kubernetes Authors.

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
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/fluxcd/cli-utils/pkg/object"
)

type KindOrder struct {
	First []string
	Last  []string
}

// ReconcileOrder holds the list of the Kubernetes native kinds that
// describes in which order they are reconciled.
var ReconcileOrder = KindOrder{
	First: []string{
		"CustomResourceDefinition",
		"Namespace",
		"ClusterClass",
		"RuntimeClass",
		"PriorityClass",
		"StorageClass",
		"VolumeSnapshotClass",
		"IngressClass",
		"GatewayClass",
		"ResourceQuota",
		"ServiceAccount",
		"Role",
		"ClusterRole",
		"RoleBinding",
		"ClusterRoleBinding",
		"ConfigMap",
		"Secret",
		"Service",
		"LimitRange",
		"Deployment",
		"StatefulSet",
		"CronJob",
		"PodDisruptionBudget",
	},
	Last: []string{
		"MutatingWebhookConfiguration",
		"ValidatingWebhookConfiguration",
	},
}

type SortableUnstructureds []*unstructured.Unstructured

var _ sort.Interface = SortableUnstructureds{}

func (a SortableUnstructureds) Len() int      { return len(a) }
func (a SortableUnstructureds) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a SortableUnstructureds) Less(i, j int) bool {
	first := object.UnstructuredToObjMetadata(a[i])
	second := object.UnstructuredToObjMetadata(a[j])
	return less(first, second)
}

type SortableMetas []object.ObjMetadata

var _ sort.Interface = SortableMetas{}

func (a SortableMetas) Len() int      { return len(a) }
func (a SortableMetas) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a SortableMetas) Less(i, j int) bool {
	return less(a[i], a[j])
}

func less(i, j object.ObjMetadata) bool {
	if !Equals(i.GroupKind, j.GroupKind) {
		return IsLessThan(i.GroupKind, j.GroupKind)
	}
	// In case of tie, compare the namespace and name combination so that the output
	// order is consistent irrespective of input order
	if i.Namespace != j.Namespace {
		return i.Namespace < j.Namespace
	}
	return i.Name < j.Name
}

func computeKind2index() map[string]int {
	// An attempt to order things to help k8s, e.g.
	// a Service should come before things that refer to it.
	// Namespace should be first.
	// In some cases order just specified to provide determinism.

	kind2indexResult := make(map[string]int, len(ReconcileOrder.First)+len(ReconcileOrder.Last))
	for i, n := range ReconcileOrder.First {
		kind2indexResult[n] = -len(ReconcileOrder.First) + i
	}
	for i, n := range ReconcileOrder.Last {
		kind2indexResult[n] = 1 + i
	}
	return kind2indexResult
}

// getIndexByKind returns the index of the kind respecting the order
func getIndexByKind(kind string) int {
	return computeKind2index()[kind]
}

func Equals(i, j schema.GroupKind) bool {
	return i.Group == j.Group && i.Kind == j.Kind
}

func IsLessThan(i, j schema.GroupKind) bool {
	indexI := getIndexByKind(i.Kind)
	indexJ := getIndexByKind(j.Kind)
	if indexI != indexJ {
		return indexI < indexJ
	}
	if i.Group != j.Group {
		return i.Group < j.Group
	}
	return i.Kind < j.Kind
}
