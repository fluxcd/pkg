/*
Copyright 2022 The Flux authors

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

package git

import (
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// SecurePath accepts an absolute or relative path and returns a path that is
// safe for use. If the path is absolute, it's `filepath.Clean`ed and returned.
// If the path is relative, it's securely joined against the working directory
// to ensure that the resultant path is a child of the working directory.
func SecurePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	joined, err := securejoin.SecureJoin(wd, path)
	if err != nil {
		return "", err
	}

	return joined, nil
}

// TransformRevision transforms a "legacy" revision string into a "new"
// revision string. It accepts the following formats:
//
//   - main/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - feature/branch/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - HEAD/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//
// Which are transformed into the following formats respectively:
//
// - main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
// - feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
// - sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//
// NOTE: This function is only intended to be used for backwards compatibility
// with the old revision format. It may be removed in a future release.
func TransformRevision(rev string) string {
	if rev == "" || strings.LastIndex(rev, ":") >= 0 {
		return rev
	}
	p, h := SplitRevision(rev)
	if p == "" {
		return h.Digest()
	}
	return p + "@" + h.Digest()
}

// SplitRevision splits a revision string into it's named pointer and hash
// components. It accepts the following formats:
//
//   - main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - main/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - feature/branch/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - HEAD/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - 5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//
// If the revision string does not contain a named pointer, the returned
// string will be empty.
func SplitRevision(rev string) (string, Hash) {
	return ExtractNamedPointerFromRevision(rev), ExtractHashFromRevision(rev)
}

// ExtractNamedPointerFromRevision extracts the named pointer from a revision
// string. It accepts the following formats:
//
//   - main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - main/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - feature/branch/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//
// If the revision string does not contain a named pointer, the returned string
// is empty.
func ExtractNamedPointerFromRevision(rev string) string {
	if i := strings.LastIndex(rev, "@"); i != -1 {
		return rev[:i]
	}
	if i := strings.LastIndex(rev, "/"); i != -1 {
		if s := rev[:i]; s != "HEAD" {
			return s
		}
	}
	return ""
}

// ExtractHashFromRevision extracts the hash from a revision string. It accepts
// the following formats:
//
//   - main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - main/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - feature/branch/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - HEAD/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - 5394cb7f48332b2de7c17dd8b8384bbc84b7e738
func ExtractHashFromRevision(rev string) Hash {
	if rev == "" {
		return nil
	}
	if i := strings.LastIndex(rev, ":"); i != -1 {
		return Hash(rev[i+1:])
	}
	if ss := strings.Split(rev, "/"); len(ss) > 1 {
		return Hash(ss[len(ss)-1])
	}
	return Hash(rev)
}
