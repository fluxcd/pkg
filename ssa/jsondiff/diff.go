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

// DiffType is the type of change detected by the server-side apply diff
// operation.
type DiffType string

const (
	// DiffTypeCreate indicates that the resource does not exist
	// and needs to be created.
	DiffTypeCreate DiffType = "create"
	// DiffTypeUpdate indicates that the resource exists and needs
	// to be updated.
	DiffTypeUpdate DiffType = "update"
	// DiffTypeExclude indicates that the resource is excluded from
	// the diff.
	DiffTypeExclude DiffType = "exclude"
	// DiffTypeNone indicates that the resource exists and is
	// identical to the dry-run object.
	DiffTypeNone DiffType = "none"
)

// Diff is a change detected by the server-side apply diff operation.
type Diff struct {
	// Type of change detected.
	Type DiffType

	// GroupVersionKind of the resource the Patch applies to.
	GroupVersionKind schema.GroupVersionKind

	// Namespace of the resource the Patch applies to.
	Namespace string

	// Name of the resource the Patch applies to.
	Name string

	// Patch with the changes detected for the resource.
	Patch jsondiff.Patch
}

// NewDiffForUnstructured creates a new Diff for the given unstructured object.
func NewDiffForUnstructured(obj *unstructured.Unstructured, t DiffType, p jsondiff.Patch) *Diff {
	return &Diff{
		Type:             t,
		GroupVersionKind: obj.GetObjectKind().GroupVersionKind(),
		Namespace:        obj.GetNamespace(),
		Name:             obj.GetName(),
		Patch:            p,
	}
}

// DiffSet is a list of changes.
type DiffSet []*Diff
