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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// DesiredObject is the client.Object that was used as the desired state to
	// generate the Patch.
	DesiredObject client.Object

	// ClusterObject is the client.Object in the cluster that was used as the
	// current state to generate the Patch.
	// It is nil if the resource does not exist in the cluster or has been
	// excluded, which can be detected by checking the Type field for the
	// value DiffTypeCreate or DiffTypeExclude.
	ClusterObject client.Object

	// Patch with the changes detected for the resource.
	Patch jsondiff.Patch
}

// GetName returns the name of the resource the Diff applies to.
func (d *Diff) GetName() string {
	return d.DesiredObject.GetName()
}

// GetNamespace returns the namespace of the resource the Diff applies to.
func (d *Diff) GetNamespace() string {
	return d.DesiredObject.GetNamespace()
}

// GroupVersionKind returns the GroupVersionKind of the resource the Diff
// applies to.
func (d *Diff) GroupVersionKind() schema.GroupVersionKind {
	return d.DesiredObject.GetObjectKind().GroupVersionKind()
}

// CopyPatch returns a copy of the Patch.
func (d *Diff) CopyPatch() jsondiff.Patch {
	patch := make(jsondiff.Patch, len(d.Patch))
	_ = copy(patch, d.Patch)
	return patch
}

// NewDiffForUnstructured creates a new Diff for the given unstructured object.
func NewDiffForUnstructured(desired, cluster client.Object, t DiffType, p jsondiff.Patch) *Diff {
	return &Diff{
		Type:          t,
		DesiredObject: desired,
		ClusterObject: cluster,
		Patch:         p,
	}
}

// DiffSet is a list of changes.
type DiffSet []*Diff

// HasType returns true if the DiffSet contains a Diff of the given type.
func (ds DiffSet) HasType(t DiffType) bool {
	for _, d := range ds {
		if d.Type == t {
			return true
		}
	}
	return false
}

// HasChanges returns true if the DiffSet contains a Diff of type
// DiffTypeCreate or DiffTypeUpdate.
func (ds DiffSet) HasChanges() bool {
	for _, d := range ds {
		if d.Type == DiffTypeCreate || d.Type == DiffTypeUpdate {
			return true
		}
	}
	return false
}
