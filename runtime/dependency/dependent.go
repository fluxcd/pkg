/*
Copyright 2026 The Flux authors

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
	"github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Dependent is a Kubernetes object that carries identity metadata
// and dependency declarations for sorting, existence verification,
// and readiness checking.
type Dependent interface {
	GetAPIVersion() string
	GetKind() string
	GetName() string
	GetNamespace() string
	meta.ObjectWithDependencies
}

// unstructuredDependent adapts an *unstructured.Unstructured to the
// Dependent interface by injecting the parsed spec.dependsOn list.
type unstructuredDependent struct {
	*unstructured.Unstructured
	deps []meta.DependencyReference
}

func (d *unstructuredDependent) GetDependsOn() []meta.DependencyReference { return d.deps }
