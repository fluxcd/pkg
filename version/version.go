/*
Copyright 2020 The Flux authors

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

package version

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// ParseVersion parses a version string and returns a semver.Version object.
// The validation is looser than the official semver spec, allowing for
// a 'v' prefix and 0-prefixed numbers in the major, minor, and patch segments
// (e.g., v2025.02.03-rc.1 is considered valid).
func ParseVersion(v string) (*semver.Version, error) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil, semver.ErrInvalidSemVer
	}

	return semver.NewVersion(v)
}

// Sort filters the given strings based on the provided semver range
// and sorts them in descending order.
func Sort(c *semver.Constraints, vs []string) []string {
	var versions []*semver.Version
	for _, v := range vs {
		if pv, err := ParseVersion(v); err == nil && (c == nil || c.Check(pv)) {
			versions = append(versions, pv)
		}
	}
	sort.Sort(sort.Reverse(semver.Collection(versions)))
	sorted := make([]string, 0, len(versions))
	for _, v := range versions {
		sorted = append(sorted, v.Original())
	}
	return sorted
}

// repositoryBaseline holds the baseline version information for a repository.
type repositoryBaseline struct {
	// major is the major version of the repository. in the flux2Minor version.
	major int

	// minor is the minor version of the repository in the flux2Minor version.
	minor int

	// flux2Minor is any flux2 minor version where the repository
	// was present in the distribution. The major and minor fields
	// represent the version of the repository in that flux2 minor
	// version.
	flux2Minor int
}

// baseline maps repository names to a reference point used for
// computing version offsets. When a new repository is added, a
// new entry MUST be added here where major and minor represent
// the repository version in the given flux2Minor. As long as
// this relationship holds, any values are fine. It doesn't have
// to be the first flux2 minor where the repository was introduced,
// as we offer no API for such use case. It hasn't been needed so
// far.
var baseline = map[string]repositoryBaseline{
	"flux2":                       {major: 2, minor: 7, flux2Minor: 7},
	"source-controller":           {major: 1, minor: 7, flux2Minor: 7},
	"kustomize-controller":        {major: 1, minor: 7, flux2Minor: 7},
	"helm-controller":             {major: 1, minor: 4, flux2Minor: 7},
	"notification-controller":     {major: 1, minor: 7, flux2Minor: 7},
	"image-reflector-controller":  {major: 1, minor: 0, flux2Minor: 7},
	"image-automation-controller": {major: 1, minor: 0, flux2Minor: 7},
	"source-watcher":              {major: 2, minor: 0, flux2Minor: 7},
}

// RepoMajor returns the major version for the given repository name.
func RepoMajor(repoName string) (int, error) {
	cv, ok := baseline[repoName]
	if !ok {
		return 0, fmt.Errorf("unknown repository %q: not in baseline mapping", repoName)
	}
	return cv.major, nil
}

// RepoMinorForFluxMinor computes the repository minor version
// for the given flux2 distribution minor version.
// This function also supports repoName="flux2".
func RepoMinorForFluxMinor(repoName string, flux2Minor int) (int, error) {
	cv, ok := baseline[repoName]
	if !ok {
		return 0, fmt.Errorf("unknown repository %q: not in baseline mapping", repoName)
	}
	return flux2Minor - cv.flux2Minor + cv.minor, nil
}

// FluxMinorForRepoMinor computes the flux2 distribution minor
// version for the given repository minor version.
// This function also supports repoName="flux2" and
// repoMinor=<any minor version of flux2>.
func FluxMinorForRepoMinor(repoName string, repoMinor int) (int, error) {
	cv, ok := baseline[repoName]
	if !ok {
		return 0, fmt.Errorf("unknown repository %q: not in baseline mapping", repoName)
	}
	return repoMinor - cv.minor + cv.flux2Minor, nil
}
