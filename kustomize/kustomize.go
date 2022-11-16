/*
Copyright 2022 The Flux authors

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

package kustomize

// SubstituteReference contains a reference to a resource containing
// the variables name and value.
type SubstituteReference struct {
	// Kind of the values referent, valid values are ('Secret', 'ConfigMap').
	Kind string `json:"kind"`

	// Name of the values referent. Should reside in the same namespace as the
	// referring resource.
	Name string `json:"name"`

	// Optional indicates whether the referenced resource must exist, or whether to
	// tolerate its absence. If true and the referenced resource is absent, proceed
	// as if the resource was present but empty, without any variables defined.
	Optional bool `json:"optional,omitempty"`
}
