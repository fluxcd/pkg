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
