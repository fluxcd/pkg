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
// NamespacedObjectReference based on their dependencies. It performs a
// topological sort using a depth-first search algorithm, which has
// runtime complexity of O(|V| + |E|), where |V| is the number of
// vertices (objects) and |E| is the number of edges (dependencies).
//
// Reference:
// https://en.wikipedia.org/wiki/Topological_sorting#Depth-first_search
func Sort(objects []Dependent) ([]meta.NamespacedObjectReference, error) {
	// Build vertices and edges.
	vertices := make([]meta.NamespacedObjectReference, 0, len(objects))
	edges := make(map[meta.NamespacedObjectReference][]meta.NamespacedObjectReference)
	for _, obj := range objects {
		u := meta.NamespacedObjectReference{
			Name:      obj.GetName(),
			Namespace: obj.GetNamespace(),
		}
		vertices = append(vertices, u)
		for _, dep := range obj.GetDependsOn() {
			v := meta.NamespacedObjectReference{
				Name:      dep.Name,
				Namespace: dep.Namespace,
			}
			if v.Namespace == "" {
				v.Namespace = obj.GetNamespace()
			}
			edges[u] = append(edges[u], v)
		}
	}

	// Compute topological order with depth-first search.
	var sorted []meta.NamespacedObjectReference
	var depthFirstSearch func(u meta.NamespacedObjectReference) (cycle []string)
	mark := make(map[meta.NamespacedObjectReference]byte)
	depthFirstSearch = func(u meta.NamespacedObjectReference) []string {
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
