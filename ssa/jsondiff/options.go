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

package jsondiff

// ResourceOption is some configuration that modifies the diffing behavior for
// a single resource.
type ResourceOption interface {
	ApplyToResource(options *ResourceOptions)
}

// ListOption is some configuration that modifies the diffing behavior for
// a set of resources.
type ListOption interface {
	ApplyToList(options *ListOptions)
}

// ResourceOptions holds options for the server-side apply diff operation.
type ResourceOptions struct {
	// FieldManager is the name of the user or component submitting
	// the server-side apply request.
	FieldOwner string
	// IgnorePaths is a list of JSON pointers to ignore when comparing objects.
	IgnorePaths []string
	// MaskSecrets is a flag to mask secrets in the diff.
	MaskSecrets bool
}

// ApplyOptions applies the given options on these options, and then returns
// itself (for convenient chaining).
func (o *ResourceOptions) ApplyOptions(opts []ResourceOption) *ResourceOptions {
	for _, opt := range opts {
		opt.ApplyToResource(o)
	}
	return o
}

// ListOptions holds options for the server-side apply diff operation.
type ListOptions struct {
	// FieldManager is the name of the user or component submitting
	// the server-side apply request.
	FieldManager string
	// ExclusionSelectors is a map of annotations or labels which mark a
	// resource to be excluded from the server-side apply diff.
	ExclusionSelectors map[string]string
	// IgnorePathSelectors is a list of selectors that match resources
	// to ignore JSON pointers in.
	IgnorePathSelectors []IgnorePathSelector
}

// ApplyOptions applies the given options on these options, and then returns
// itself (for convenient chaining).
func (o *ListOptions) ApplyOptions(opts []ListOption) *ListOptions {
	for _, opt := range opts {
		opt.ApplyToList(o)
	}
	return o
}

// FieldOwner sets the field manager for the server-side apply request.
type FieldOwner string

// ApplyToResource applies this configuration to the given options.
func (f FieldOwner) ApplyToResource(opts *ResourceOptions) {
	opts.FieldOwner = string(f)
}

// ApplyToList applies this configuration to the given options.
func (f FieldOwner) ApplyToList(opts *ListOptions) {
	opts.FieldManager = string(f)
}

// ExclusionSelector sets the annotations or labels which mark a resource to
// be excluded from the server-side apply diff.
type ExclusionSelector map[string]string

// ApplyToList applies this configuration to the given options.
func (e ExclusionSelector) ApplyToList(opts *ListOptions) {
	opts.ExclusionSelectors = e
}

// IgnorePaths sets the JSON pointers to ignore when comparing objects.
type IgnorePaths []string

// ApplyToResource applies this configuration to the given options.
func (i IgnorePaths) ApplyToResource(opts *ResourceOptions) {
	opts.IgnorePaths = i
}

// IgnorePathSelectors sets the JSON pointers to ignore when comparing objects.
type IgnorePathSelectors []IgnorePathSelector

// ApplyToList applies this configuration to the given options.
func (i IgnorePathSelectors) ApplyToList(opts *ListOptions) {
	opts.IgnorePathSelectors = i
}

// MaskSecrets sets the flag to mask secrets in the diff.
type MaskSecrets bool

// ApplyToResource applies this configuration to the given options.
func (m MaskSecrets) ApplyToResource(opts *ResourceOptions) {
	opts.MaskSecrets = bool(m)
}
