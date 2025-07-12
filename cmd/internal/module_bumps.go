/*
Copyright 2025 The Flux authors

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

package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ModuleBumps represents a collection of module bumps
// that need to be applied to the repository.
type ModuleBumps struct {
	// bumps are the Go modules that need to be released because
	// there is a diff between the latest tag and HEAD.
	bumps []*ModuleBump

	// targetModules[i][j] is the j-th module that needs to receive
	// the i-th bump, i.e. bumps[i].
	targetModules [][]string

	// mustBumpInternalModules is true if at least one module needs to receive at least one bump.
	mustBumpInternalModules bool

	// tagsToPush are the tags that need to be pushed to the remote repository
	// in the topological order.
	tagsToPush []string
}

// MustBumpInternalModules returns true if there are any module bumps that need to be applied
// to the current repository.
func (m *ModuleBumps) MustBumpInternalModules() bool {
	return m.mustBumpInternalModules
}

// PrintBumps prints the module bumps result in a human-readable format.
func (m *ModuleBumps) PrintBumps() {
	for i, bump := range m.bumps {
		if len(m.targetModules[i]) == 0 {
			continue
		}
		fmt.Printf("Bumped %s in modules: %s\n", bump, strings.Join(m.targetModules[i], ", "))
	}
}

// ApplyInternalBumps applies the module bumps to the file system.
func (m *ModuleBumps) ApplyInternalBumps(ctx context.Context) error {
	for i, bump := range m.bumps {
		for _, targetModule := range m.targetModules[i] {
			if _, err := bump.Apply(ctx, targetModule); err != nil {
				return fmt.Errorf("failed to apply bump %s to module %s: %w", bump, targetModule, err)
			}
		}
	}
	return nil
}

// MustPushTags tells whether the tags need to be pushed to the remote repository.
func (m *ModuleBumps) MustPushTags() bool {
	return len(m.tagsToPush) > 0
}

// PrintTags prints the tags that will be pushed to the remote repository.
func (m *ModuleBumps) PrintTags() {
	for _, tag := range m.tagsToPush {
		fmt.Println(tag)
	}
}

// PushTags pushes the tags to the remote repository.
func (m *ModuleBumps) PushTags(ctx context.Context) error {
	for _, tag := range m.tagsToPush {
		tagCmd := exec.CommandContext(ctx, "git", "tag", "-s", "-m", tag, tag)
		tagCmd.Stdout = os.Stdout
		tagCmd.Stderr = os.Stderr
		if err := tagCmd.Run(); err != nil {
			return fmt.Errorf("failed to create tag %s: %w", tag, err)
		}
		pushCmd := exec.CommandContext(ctx, "git", "push", "origin", tag)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("failed to push tag %s: %w", tag, err)
		}
	}
	return nil
}
