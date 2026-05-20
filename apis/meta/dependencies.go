/*
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

package meta

import (
	"strconv"
	"strings"
	"unicode"
)

// ObjectWithDependencies describes a Kubernetes resource object with dependencies.
// +k8s:deepcopy-gen=false
type ObjectWithDependencies interface {
	// GetDependsOn returns a DependencyReference list the object depends on.
	GetDependsOn() []DependencyReference
}

// MakeDependsOn parses a list of dependency strings into DependencyReference
// objects. Each dependency string can be in one of the following formats:
//   - "name" - a Flux Applier API (Kustomization or HelmRelease) dependency in the same namespace
//   - "namespace/name" - a Flux Applier API (Kustomization or HelmRelease) dependency in a specific namespace
//   - "name@readyExpr" - a Flux Applier API (Kustomization or HelmRelease) dependency with a CEL readiness expression (enabled)
//   - "namespace/name@readyExpr" - a Flux Applier API (Kustomization or HelmRelease) dependency in a specific namespace with a CEL expression (enabled)
//   - "apiVersion/Kind/name" - a Kubernetes resource dependency in the same namespace
//   - "apiVersion/Kind/name@readyExpr" - a Kubernetes resource dependency with a CEL readiness expression (disabled)
//   - "apiVersion/Kind/name:true@readyExpr" - a Kubernetes resource dependency with a CEL readiness expression (enabled)
//   - "apiVersion/Kind/namespace/name" - a Kubernetes resource dependency in a specific namespace
//   - "apiVersion/Kind/namespace/name@readyExpr" - a Kubernetes resource dependency in a specific namespace with a CEL readiness expression (disabled)
//   - "apiVersion/Kind/namespace/name:true@readyExpr" - a Kubernetes resource dependency in a specific namespace with a CEL readiness expression (enabled)
//
// The : symbol is used to separate the resource reference from the readiness check enablement.
// The @ symbol is used to separate the resource reference from the CEL expression.
// Note that : and @ cannot be part of resource names or namespaces per Kubernetes naming conventions:
// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
// For CEL expression syntax, see:
// https://github.com/google/cel-spec/blob/master/doc/langdef.md
func MakeDependsOn(deps []string) []DependencyReference {
	refs := make([]DependencyReference, 0, len(deps))
	for _, dep := range deps {
		ref := DependencyReference{}

		// Split off the CEL ready expression if present.
		if idx := strings.Index(dep, "@"); idx != -1 {
			ref.ReadyExpr = dep[idx+1:]
			dep = dep[:idx]
		}

		// Split off the readiness check boolean value if present.
		if idx := strings.Index(dep, ":"); idx != -1 {
			ready, err := strconv.ParseBool(dep[idx+1:])
			if err != nil {
				ready = false
			}
			ref.Ready = new(ready)
			dep = dep[:idx]
		}

		// Parse the apiVersion/Kind/namespace/name dependency string into DependencyReference objects.
		parts := strings.SplitN(dep, "/", 5)
		switch len(parts) {
		case 5:
			ref.APIVersion = parts[0] + "/" + parts[1]
			ref.Kind = parts[2]
			ref.Namespace = parts[3]
			ref.Name = parts[4]
		case 4:
			// parts[1] starts with uppercase → core API ("v1/Pod/namespace/name")
			// parts[1] starts with lowercase → group/version ("apps/v1/Deployment/name")
			if unicode.IsUpper(rune(parts[1][0])) {
				ref.APIVersion = parts[0]
				ref.Kind = parts[1]
				ref.Namespace = parts[2]
				ref.Name = parts[3]
			} else {
				ref.APIVersion = parts[0] + "/" + parts[1]
				ref.Kind = parts[2]
				ref.Name = parts[3]
			}
		case 3:
			ref.APIVersion = parts[0]
			ref.Kind = parts[1]
			ref.Name = parts[2]
		case 2:
			ref.Namespace = parts[0]
			ref.Name = parts[1]
		case 1:
			ref.Name = dep
		}

		refs = append(refs, ref)
	}
	return refs
}
