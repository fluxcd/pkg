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
	"slices"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/format/diff"
)

// ComputeModuleBumps looks at the current Git repository and computes
// the necessary module bumps based on the latest tags and changes in
// the codebase. It returns the modules that need to be bumped and their
// tags in the correct push order.
func ComputeModuleBumps(ctx context.Context) (*ModuleBumps, error) {
	// Enumerate taggable and bumpable modules in the repository.
	taggables, err := EnumerateTaggableModules()
	if err != nil {
		return nil, fmt.Errorf("failed to enumerate taggable modules: %w", err)
	}
	bumpables := EnumerateBumpableModules(taggables)
	isTaggable := make(map[string]bool)
	for _, bumpable := range bumpables {
		if slices.Contains(taggables, bumpable) {
			isTaggable[bumpable] = true
		}
	}

	// Open the current Git repository.
	repo, err := git.PlainOpen(".")
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Get iterator for tags in the repository.
	tagsIter, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer tagsIter.Close()

	// Collect tags for each module.
	moduleTags := make(map[string][]*semver.Version)
	err = tagsIter.ForEach(func(ref *plumbing.Reference) error {
		tag := ref.Name().Short()
		for _, module := range taggables {
			prefix := module + "/v"
			if !strings.HasPrefix(tag, prefix) {
				continue
			}
			v, err := semver.NewVersion(strings.TrimPrefix(tag, prefix))
			if err != nil {
				continue
			}
			moduleTags[module] = append(moduleTags[module], v)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate over tags: %w", err)
	}

	// Find latest for each module.
	moduleLatest := make(map[string]*semver.Version)
	for module, versions := range moduleTags {
		sort.Sort(sort.Reverse(semver.Collection(versions)))
		moduleLatest[module] = versions[0]
	}

	// Find modules that have a diff between the latest tag and HEAD,
	// i.e. those that need to be bumped.
	var moduleBumps []*ModuleBump
	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
	}
	headCommit, err := repo.CommitObject(headRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD commit: %w", err)
	}
	for _, taggable := range taggables {
		// Is it a new module?
		latest, ok := moduleLatest[taggable]
		if !ok {
			bump, err := NewModuleBumpForNewModule(taggable)
			if err != nil {
				return nil, fmt.Errorf("failed to create module bump for new module %s: %w", taggable, err)
			}
			moduleBumps = append(moduleBumps, bump)
			continue
		}

		// Compute the patch (the diff) between the latest tag and HEAD.
		tag := fmt.Sprintf("%s/v%s", taggable, latest.Original())
		tagRef, err := repo.Tag(tag)
		if err != nil {
			return nil, fmt.Errorf("failed to get tag %s: %w", tag, err)
		}
		tagObject, err := repo.TagObject(tagRef.Hash())
		if err != nil {
			return nil, fmt.Errorf("failed to get commit for tag %s: %w", tag, err)
		}
		tagCommit, err := repo.CommitObject(tagObject.Target)
		if err != nil {
			return nil, fmt.Errorf("failed to get commit for tag %s: %w", tag, err)
		}
		patch, err := tagCommit.PatchContext(ctx, headCommit)
		if err != nil {
			return nil, fmt.Errorf("failed to create patch between %s and HEAD: %w", tag, err)
		}

		// For each file change in the patch, check if it belongs to the module.
		prefix := taggable + "/"
		fileChanged := func(f diff.File) bool {
			if f == nil || !strings.HasPrefix(f.Path(), prefix) {
				return false
			}

			// This loop is for removing bumps to <taggable> if the file
			// is of the form <taggable>/<other_module>/<file> where
			// <bumpable> is <taggable>/<other_module>.
			path := f.Path()
			for _, bumpable := range bumpables {
				if strings.HasPrefix(path, bumpable+"/") && strings.HasPrefix(bumpable, prefix) {
					return false
				}
			}

			return true
		}
		changed := false
		for _, file := range patch.FilePatches() {
			from, to := file.Files()
			if fileChanged(from) || fileChanged(to) {
				changed = true
				break
			}
		}
		if changed {
			bump, err := NewModuleBump(taggable, latest.Original(), latest.IncMinor().String())
			if err != nil {
				return nil, fmt.Errorf("failed to create module bump for %s: %w", taggable, err)
			}
			moduleBumps = append(moduleBumps, bump)
		}
	}

	// For each taggable module that needs to receive a new release,
	// bump it inside other bumpable modules.
	targetModules := make([][]string, len(moduleBumps))
	mustBumpInternalModules := false
	for i := 0; i < len(moduleBumps); i++ { // moduleBumps grows dynamically inside this loop.
		bump := moduleBumps[i]
		for _, targetModule := range bumpables {
			if targetModule == bump.module {
				continue
			}
			ok, err := bump.DryRunApply(ctx, targetModule)
			if err != nil {
				return nil, fmt.Errorf("failed to apply bump %s to module %s: %w", bump, targetModule, err)
			}
			if ok {
				targetModules[i] = append(targetModules[i], targetModule)

				// After updating the targetModule, if targetModule is taggable,
				// then it must be bumped as well.
				if !isTaggable[targetModule] {
					continue
				}
				willBeBumpedAlready := false
				for _, existingBump := range moduleBumps {
					if targetModule == existingBump.module {
						willBeBumpedAlready = true
						break
					}
				}
				if !willBeBumpedAlready {
					var newBump *ModuleBump
					latest, ok := moduleLatest[targetModule]
					if !ok {
						// This is a new module that was never tagged before.
						newBump, err = NewModuleBumpForNewModule(targetModule)
						if err != nil {
							return nil, fmt.Errorf("failed to create module bump for new module %s: %w", targetModule, err)
						}
					} else {
						// This is an existing module that already has a tag.
						newBump, err = NewModuleBump(targetModule, latest.Original(), latest.IncMinor().String())
						if err != nil {
							return nil, fmt.Errorf("failed to create module bump for %s: %w", targetModule, err)
						}
					}
					moduleBumps = append(moduleBumps, newBump)
					targetModules = append(targetModules, nil)
				}
			}
		}

		if len(targetModules[i]) > 0 {
			mustBumpInternalModules = true
		}
	}

	// If bumps must be applied, return early without computing the order of tags to push.
	if mustBumpInternalModules {
		return &ModuleBumps{
			bumps:                   moduleBumps,
			targetModules:           targetModules,
			mustBumpInternalModules: true,
		}, nil
	}

	// Compute topological order of tags to push using a depth-first search.
	// https://en.wikipedia.org/wiki/Topological_sorting#Depth-first_search
	const (
		unmarked = iota
		permanentMark
		temporaryMark
	)
	mark := make([]int, len(moduleBumps))
	tagsToPush := make([]string, 0, len(moduleBumps))
	var depthFirstSearch func(i int) // Recursive closures need to be defined like this.
	depthFirstSearch = func(i int) {
		if mark[i] == permanentMark {
			return
		}
		if mark[i] == temporaryMark {
			// Should never happen, as cycles are not allowed in Go modules dependencies.
			panic("cycle detected in module bumps")
		}
		mark[i] = temporaryMark
		for _, targetModule := range targetModules[i] {
			// Find which j represents targetModule in moduleBumps.
			for j, targetBump := range moduleBumps {
				if targetBump.module == targetModule {
					depthFirstSearch(j)
					break
				}
			}
		}
		mark[i] = permanentMark
		tag := fmt.Sprintf("%s/v%s", moduleBumps[i].module, moduleBumps[i].newVersion)
		tagsToPush = append(tagsToPush, tag)
	}
	for i := range moduleBumps {
		if mark[i] == unmarked {
			depthFirstSearch(i)
		}
	}

	return &ModuleBumps{
		bumps:                   moduleBumps,
		targetModules:           targetModules,
		mustBumpInternalModules: false,
		tagsToPush:              tagsToPush,
	}, nil
}
