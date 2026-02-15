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

package internal

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// collectLatestReachableTags returns the latest semver version for each module
// by examining tags reachable from the given commit in the repository.
// Tags are expected to have the format "<module>/v<version>".
func collectLatestReachableTags(repo *git.Repository, head plumbing.Hash) (map[string]*semver.Version, error) {
	// Build set of all commits reachable from head.
	commitIter, err := repo.Log(&git.LogOptions{From: head})
	if err != nil {
		return nil, fmt.Errorf("failed to get commit log: %w", err)
	}
	defer commitIter.Close()
	reachable := make(map[plumbing.Hash]struct{})
	err = commitIter.ForEach(func(c *object.Commit) error {
		reachable[c.Hash] = struct{}{}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate commits: %w", err)
	}

	// Iterate tags and collect versions per module.
	tagsIter, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to get tags: %w", err)
	}
	defer tagsIter.Close()

	moduleTags := make(map[string][]*semver.Version)
	err = tagsIter.ForEach(func(ref *plumbing.Reference) error {
		tag := ref.Name().Short()

		// Resolve tag to its target commit and skip if not reachable.
		var targetHash plumbing.Hash
		if tagObj, err := repo.TagObject(ref.Hash()); err == nil {
			targetHash = tagObj.Target
		} else {
			targetHash = ref.Hash()
		}
		if _, ok := reachable[targetHash]; !ok {
			return nil
		}

		// Parse tag format: <module>/v<version>
		idx := strings.LastIndex(tag, "/v")
		if idx < 0 {
			return nil
		}
		module := tag[:idx]
		v, err := semver.NewVersion(tag[idx+1:])
		if err != nil {
			return nil
		}
		moduleTags[module] = append(moduleTags[module], v)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate over tags: %w", err)
	}

	// Find latest version for each module.
	latest := make(map[string]*semver.Version)
	for module, versions := range moduleTags {
		sort.Sort(sort.Reverse(semver.Collection(versions)))
		latest[module] = versions[0]
	}
	return latest, nil
}
