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

import (
	"github.com/wI2L/jsondiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// ChangeType is the type of change detected by the server-side apply diff
// operation.
type ChangeType string

const (
	// ChangeTypeCreate indicates that the resource does not exist
	// and needs to be created.
	ChangeTypeCreate ChangeType = "create"
	// ChangeTypeUpdate indicates that the resource exists and needs
	// to be updated.
	ChangeTypeUpdate ChangeType = "update"
	// ChangeTypeExclude indicates that the resource is excluded from
	// the diff.
	ChangeTypeExclude ChangeType = "exclude"
	// ChangeTypeNone indicates that the resource exists and is
	// identical to the dry-run object.
	ChangeTypeNone ChangeType = "none"
)

// Change is a change detected by the server-side apply diff operation.
type Change struct {
	// Type of change detected.
	Type ChangeType

	// GroupVersionKind of the resource the Patch applies to.
	GroupVersionKind schema.GroupVersionKind

	// Namespace of the resource the Patch applies to.
	Namespace string

	// Name of the resource the Patch applies to.
	Name string

	// Patch with the changes detected for the resource.
	Patch jsondiff.Patch
}

// NewChangeForUnstructured creates a new Change for the given unstructured object.
func NewChangeForUnstructured(obj *unstructured.Unstructured, t ChangeType, p jsondiff.Patch) *Change {
	return &Change{
		Type:             t,
		GroupVersionKind: obj.GetObjectKind().GroupVersionKind(),
		Namespace:        obj.GetNamespace(),
		Name:             obj.GetName(),
		Patch:            p,
	}
}

// ChangeSet is a list of changes.
type ChangeSet []*Change
