/*
Copyright 2021 Stefan Prodan
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

package ssa

import (
	"fmt"
	"strings"

	"github.com/fluxcd/cli-utils/pkg/object"
)

// Action represents the action type performed by the reconciliation process.
type Action string

// String returns the string representation of the action.
func (a Action) String() string {
	return string(a)
}

const (
	// CreatedAction represents the creation of a new object.
	CreatedAction Action = "created"
	// ConfiguredAction represents the update of an existing object.
	ConfiguredAction Action = "configured"
	// UnchangedAction represents the absence of any action to an object.
	UnchangedAction Action = "unchanged"
	// DeletedAction represents the deletion of an object.
	DeletedAction Action = "deleted"
	// SkippedAction represents the fact that no action was performed on an object
	// due to the object being excluded from the reconciliation.
	SkippedAction Action = "skipped"
	// UnknownAction represents an unknown action.
	UnknownAction Action = "unknown"
)

// ChangeSet holds the result of the reconciliation of an object collection.
type ChangeSet struct {
	Entries []ChangeSetEntry
}

// NewChangeSet returns a ChangeSet will an empty slice of entries.
func NewChangeSet() *ChangeSet {
	return &ChangeSet{Entries: []ChangeSetEntry{}}
}

// Add appends the given entry to the end of the slice.
func (c *ChangeSet) Add(e ChangeSetEntry) {
	c.Entries = append(c.Entries, e)
}

// Append adds the given ChangeSet entries to end of the slice.
func (c *ChangeSet) Append(e []ChangeSetEntry) {
	c.Entries = append(c.Entries, e...)
}

func (c *ChangeSet) String() string {
	var b strings.Builder
	for _, entry := range c.Entries {
		b.WriteString(entry.String() + "\n")
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (c *ChangeSet) ToMap() map[string]Action {
	res := make(map[string]Action, len(c.Entries))
	for _, entry := range c.Entries {
		res[entry.Subject] = entry.Action
	}
	return res
}

func (c *ChangeSet) ToObjMetadataSet() object.ObjMetadataSet {
	var res []object.ObjMetadata
	for _, entry := range c.Entries {
		res = append(res, entry.ObjMetadata)
	}
	return res
}

// ChangeSetEntry defines the result of an action performed on an object.
type ChangeSetEntry struct {
	// ObjMetadata holds the unique identifier of this entry.
	ObjMetadata object.ObjMetadata

	// GroupVersion holds the API group version of this entry.
	GroupVersion string

	// Subject represents the Object ID in the format 'kind/namespace/name'.
	Subject string

	// Action represents the action type taken by the reconciler for this object.
	Action Action
}

func (e ChangeSetEntry) String() string {
	return fmt.Sprintf("%s %s", e.Subject, e.Action)
}
