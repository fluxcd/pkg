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
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/fluxcd/pkg/version"
)

const pkgRepoURL = "https://github.com/fluxcd/pkg.git"

// depUpdate represents a single dependency version change.
type depUpdate struct {
	module     string
	oldVersion string
	newVersion string
}

// BumpResult holds the outcome of a BumpDeps operation.
type BumpResult struct {
	pkgBranch string
	updates   []depUpdate
}

// NothingToUpdate reports whether there are no dependency updates.
func (r *BumpResult) NothingToUpdate() bool {
	return len(r.updates) == 0
}

// PrintSummary prints a human-readable summary of the bump result.
func (r *BumpResult) PrintSummary() {
	fmt.Printf("pkg branch: %s\n", r.pkgBranch)
	if r.NothingToUpdate() {
		fmt.Println("All fluxcd/pkg dependencies are up to date.")
		return
	}
	fmt.Println("Updates:")
	for _, u := range r.updates {
		fmt.Printf("  github.com/fluxcd/pkg/%s: %s => %s\n", u.module, u.oldVersion, u.newVersion)
	}
}

// BumpDeps detects the current branch, maps it to the corresponding
// fluxcd/pkg branch, fetches the latest module versions from that branch,
// and updates go.mod accordingly. When preReleasePkg is true, main branches
// use flux/v2.8.x instead of main (temporary workaround for Flux 2.8).
func BumpDeps(ctx context.Context, repoPath string, preReleasePkg bool) (*BumpResult, error) {
	localBranch, err := detectLocalBranch(repoPath)
	if err != nil {
		return nil, err
	}
	fmt.Println("Local branch:", localBranch)

	controllerName, err := detectControllerName(repoPath)
	if err != nil {
		return nil, err
	}
	fmt.Println("Controller:", controllerName)

	pkgBranch, err := mapToPkgBranch(localBranch, controllerName, preReleasePkg)
	if err != nil {
		return nil, err
	}
	fmt.Println("pkg branch:", pkgBranch)

	latestVersions, err := fetchLatestVersions(ctx, pkgBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch latest versions from pkg branch %s: %w", pkgBranch, err)
	}

	var allUpdates []depUpdate

	// Bump go.mod files in dependency order: api/ first (root depends on it),
	// then root, then tests/integration/ (may depend on root).
	// go mod tidy is not run here; callers handle it separately.
	modules := []string{"api", ".", "tests/integration"}
	for _, mod := range modules {
		dir := repoPath
		if mod != "." {
			dir = repoPath + "/" + mod
			if _, err := os.Stat(dir + "/go.mod"); err != nil {
				continue
			}
		}
		fmt.Printf("Bumping %s/go.mod ...\n", mod)
		updates, err := replaceGoModVersions(dir, latestVersions)
		if err != nil {
			return nil, fmt.Errorf("failed to update %s/go.mod: %w", mod, err)
		}
		allUpdates = append(allUpdates, updates...)
	}

	return &BumpResult{
		pkgBranch: pkgBranch,
		updates:   allUpdates,
	}, nil
}

// detectLocalBranch opens the current directory as a git repo and returns the branch name.
func detectLocalBranch(repoPath string) (string, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("failed to open repository: %w", err)
	}
	headRef, err := repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD reference: %w", err)
	}
	if !headRef.Name().IsBranch() {
		return "", fmt.Errorf("HEAD is not a branch")
	}
	return headRef.Name().Short(), nil
}

// gomodModuleRegex extracts the module path from a go.mod file.
var gomodModuleRegex = regexp.MustCompile(`(?m)^module\s+(\S+)`)

// detectControllerName reads the target repo's go.mod and extracts the
// controller name (e.g. "helm-controller") from the module path.
func detectControllerName(repoPath string) (string, error) {
	gomod := fmt.Sprintf("%s/go.mod", repoPath)
	b, err := os.ReadFile(gomod)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", gomod, err)
	}
	m := gomodModuleRegex.FindStringSubmatch(string(b))
	if m == nil {
		return "", fmt.Errorf("failed to find module path in %s", gomod)
	}
	modulePath := m[1]
	// Strip version suffix (e.g. "github.com/fluxcd/flux2/v2" → "github.com/fluxcd/flux2").
	if idx := strings.LastIndex(modulePath, "/v"); idx >= 0 {
		if _, err := strconv.Atoi(modulePath[idx+2:]); err == nil {
			modulePath = modulePath[:idx]
		}
	}
	// Extract the last path component (e.g. "github.com/fluxcd/helm-controller" → "helm-controller").
	name := modulePath[strings.LastIndex(modulePath, "/")+1:]
	return name, nil
}

// releaseBranchRegex matches branch names like "release/v1.5.x" or "release/v2.3.x".
var releaseBranchRegex = regexp.MustCompile(`^release/v\d+\.(\d+)\.x$`)

// mapToPkgBranch maps a caller repo branch to the corresponding fluxcd/pkg
// branch using the controller's baseline minor version offset.
// This function also works for controllerName="flux2" and
// controllerBranch=<flux2 repo branch>.
func mapToPkgBranch(controllerBranch, controllerName string, preReleasePkg bool) (string, error) {
	switch {
	case preReleasePkg: // TODO: remove after 2.8.0 is released
		return "flux/v2.8.x", nil
	case controllerBranch == "main":
		return "main", nil
	}
	m := releaseBranchRegex.FindStringSubmatch(controllerBranch)
	if m == nil {
		fmt.Printf("Warning: branch %q does not match expected patterns, defaulting to main\n", controllerBranch)
		return "main", nil
	}
	controllerBranchMinor, _ := strconv.Atoi(m[1])
	fluxMinor, err := version.FluxMinorForRepoMinor(controllerName, controllerBranchMinor)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("flux/v2.%d.x", fluxMinor), nil
}

// fetchLatestVersions clones the pkg repo in memory for the given branch
// and returns a map of module name to latest version tag (e.g. "auth" → "v0.5.0").
func fetchLatestVersions(ctx context.Context, pkgBranch string) (map[string]string, error) {
	fmt.Printf("Cloning %s (branch %s) ...\n", pkgRepoURL, pkgBranch)
	repo, err := git.CloneContext(ctx, memory.NewStorage(), nil, &git.CloneOptions{
		URL:           pkgRepoURL,
		ReferenceName: plumbing.NewBranchReferenceName(pkgBranch),
		SingleBranch:  true,
		Tags:          git.AllTags,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to clone pkg repo: %w", err)
	}

	headRef, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD reference: %w", err)
	}

	moduleLatest, err := collectLatestReachableTags(repo, headRef.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to collect latest reachable tags: %w", err)
	}

	// Convert *semver.Version to "v<version>" strings.
	latestVersions := make(map[string]string, len(moduleLatest))
	for module, v := range moduleLatest {
		latestVersions[module] = "v" + v.String()
	}
	return latestVersions, nil
}

// gomodPkgDepRegex matches lines like: 	github.com/fluxcd/pkg/runtime v1.2.0
var gomodPkgDepRegex = regexp.MustCompile(`(?m)^\s+(github\.com/fluxcd/pkg/(\S+))\s+(v\S+)`)

// replaceGoModVersions reads go.mod, replaces fluxcd/pkg dependency versions
// with the latest ones, and writes the file back.
func replaceGoModVersions(dir string, latestVersions map[string]string) ([]depUpdate, error) {
	gomod := dir + "/go.mod"
	b, err := os.ReadFile(gomod)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", gomod, err)
	}
	oldContent := string(b)

	var updates []depUpdate
	newContent := gomodPkgDepRegex.ReplaceAllStringFunc(oldContent, func(match string) string {
		sub := gomodPkgDepRegex.FindStringSubmatch(match)
		// sub[0] = full match (with leading whitespace)
		// sub[1] = full module path, sub[2] = module name, sub[3] = current version
		module := sub[2]
		oldVersion := sub[3]
		newVersion, ok := latestVersions[module]
		if !ok || newVersion == oldVersion {
			return match
		}
		updates = append(updates, depUpdate{
			module:     module,
			oldVersion: oldVersion,
			newVersion: newVersion,
		})
		// Preserve leading whitespace from the original match.
		ws := match[:strings.Index(match, sub[1])]
		return ws + sub[1] + " " + newVersion
	})

	if len(updates) == 0 {
		return updates, nil
	}

	if err := os.WriteFile(gomod, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write %s: %w", gomod, err)
	}
	return updates, nil
}
