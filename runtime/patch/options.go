/*
Copyright 2020 The Kubernetes Authors.
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

This file is modified from the source at
https://github.com/kubernetes-sigs/cluster-api/tree/d2faf482116114c4075da1390d905742e524ff89/util/patch/options.go,
and initially adapted to work with the `conditions` package and `metav1.Condition` types.
*/

package patch

// Option is some configuration that modifies options for a patch request.
type Option interface {
	// ApplyToHelper applies this configuration to the given Helper options.
	ApplyToHelper(*HelperOptions)
}

// HelperOptions contains options for patch options.
type HelperOptions struct {
	// IncludeStatusObservedGeneration sets the status.observedGeneration field on the incoming object to match
	// metadata.generation, only if there is a change.
	IncludeStatusObservedGeneration bool

	// ForceOverwriteConditions allows the patch helper to overwrite conditions in case of conflicts.
	// This option should only ever be set in controller managing the object being patched.
	ForceOverwriteConditions bool

	// OwnedConditions defines condition types owned by the controller.
	// In case of conflicts for the owned conditions, the patch helper will always use the value provided by the
	// controller.
	OwnedConditions []string

	// FieldOwner defines the field owner configuration for Kubernetes patch operations.
	FieldOwner string
}

// WithForceOverwriteConditions allows the patch helper to overwrite conditions in case of conflicts.
// This option should only ever be set in controller managing the object being patched.
type WithForceOverwriteConditions struct{}

// ApplyToHelper applies this configuration to the given HelperOptions.
func (w WithForceOverwriteConditions) ApplyToHelper(in *HelperOptions) {
	in.ForceOverwriteConditions = true
}

// WithStatusObservedGeneration sets the status.observedGeneration field on the incoming object to match
// metadata.generation, only if there is a change.
type WithStatusObservedGeneration struct{}

// ApplyToHelper applies this configuration to the given HelperOptions.
func (w WithStatusObservedGeneration) ApplyToHelper(in *HelperOptions) {
	in.IncludeStatusObservedGeneration = true
}

// WithOwnedConditions allows to define condition types owned by the controller.
// In case of conflicts for the owned conditions, the patch helper will always use the value provided by the controller.
type WithOwnedConditions struct {
	Conditions []string
}

// ApplyToHelper applies this configuration to the given HelperOptions.
func (w WithOwnedConditions) ApplyToHelper(in *HelperOptions) {
	in.OwnedConditions = w.Conditions
}

// WithFieldOwner set the field manager name for the patch operations.
type WithFieldOwner string

// ApplyToHelper applies this configuration to the given HelperOptions.
func (w WithFieldOwner) ApplyToHelper(in *HelperOptions) {
	in.FieldOwner = string(w)
}
