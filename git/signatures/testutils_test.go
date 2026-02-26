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

package signatures_test

import (
	"os"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// parseCommitFromFixture parses a git commit object from a fixture file
func parseCommitFromFixture(fixturePath string) (*object.Commit, error) {
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		return nil, err
	}

	// Create a MemoryObject and write the commit data to it
	obj := &plumbing.MemoryObject{}
	obj.SetType(plumbing.CommitObject)
	if _, err := obj.Write(data); err != nil {
		return nil, err
	}

	// Decode the commit object
	commit := &object.Commit{}
	if err := commit.Decode(obj); err != nil {
		return nil, err
	}

	return commit, nil
}

// parseTagFromFixture parses a git tag object from a fixture file
func parseTagFromFixture(fixturePath string) (*object.Tag, error) {
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		return nil, err
	}

	// Create a MemoryObject and write the tag data to it
	obj := &plumbing.MemoryObject{}
	obj.SetType(plumbing.TagObject)
	if _, err := obj.Write(data); err != nil {
		return nil, err
	}

	// Decode the tag object
	tag := &object.Tag{}
	if err := tag.Decode(obj); err != nil {
		return nil, err
	}

	return tag, nil
}
