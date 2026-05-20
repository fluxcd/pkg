/*
Copyright 2020 The Flux authors

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

package dependency

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fluxcd/pkg/apis/meta"
)

// Dependent interface defines methods that a Kubernetes resource object should
// implement in order to use the dependency package for ordering dependencies.
type Dependent interface {
	GetAPIVersion() string
	GetKind() string
	GetName() string
	GetNamespace() string
	meta.ObjectWithDependencies
}

const (
	unmarked = iota
	permanentMark
	temporaryMark
)

// Sort takes a slice of Dependent objects and returns a sorted slice of
// TypedNamespacedObjectReference based on their dependencies. It performs a
// topological sort using a depth-first search algorithm, which has
// runtime complexity of O(|V| + |E|), where |V| is the number of
// vertices (objects) and |E| is the number of edges (dependencies).
//
// Reference:
// https://en.wikipedia.org/wiki/Topological_sorting#Depth-first_search
func Sort(objects []Dependent) ([]meta.TypedNamespacedObjectReference, error) {
	// Build vertices and edges.
	vertices := make([]meta.TypedNamespacedObjectReference, 0, len(objects))
	edges := make(map[meta.TypedNamespacedObjectReference][]meta.TypedNamespacedObjectReference)
	for _, obj := range objects {
		u := meta.TypedNamespacedObjectReference{
			APIVersion: obj.GetAPIVersion(),
			Kind:       obj.GetKind(),
			Name:       obj.GetName(),
			Namespace:  obj.GetNamespace(),
		}
		vertices = append(vertices, u)
		for _, depRef := range obj.GetDependsOn() {
			v := meta.TypedNamespacedObjectReference{
				APIVersion: depRef.APIVersion,
				Kind:       depRef.Kind,
				Name:       depRef.Name,
				Namespace:  depRef.Namespace,
			}
			// Default the namespace only when the dependency has the same Kind
			// as the parent object.
			if v.Namespace == "" && (v.Kind == "" || v.Kind == obj.GetKind()) {
				v.Namespace = obj.GetNamespace()
			}
			edges[u] = append(edges[u], v)
		}
	}

	// Compute topological order with depth-first search.
	var sorted []meta.TypedNamespacedObjectReference
	var depthFirstSearch func(u meta.TypedNamespacedObjectReference) (cycle []string)
	mark := make(map[meta.TypedNamespacedObjectReference]byte)
	depthFirstSearch = func(u meta.TypedNamespacedObjectReference) []string {
		mark[u] = temporaryMark
		for _, v := range edges[u] {
			if mark[v] == permanentMark {
				continue
			}
			if mark[v] == temporaryMark {
				// Cycle detected.
				return []string{v.String(), u.String()}
			}
			if cycle := depthFirstSearch(v); len(cycle) > 0 {
				return append(cycle, u.String())
			}
		}
		mark[u] = permanentMark
		sorted = append(sorted, u)
		return nil
	}
	for _, u := range vertices {
		if mark[u] == unmarked {
			if cycle := depthFirstSearch(u); len(cycle) > 0 {
				slices.Reverse(cycle)
				return nil, fmt.Errorf("circular dependency detected: %v", strings.Join(cycle, " -> "))
			}
		}
	}

	return sorted, nil
}
