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

// ExtractHashFromRevision extracts the hash from a revision string. It accepts
// the following formats:
//
//   - main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - main/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - feature/branch/5394cb7f48332b2de7c17dd8b8384bbc84b7e738
//   - 5394cb7f48332b2de7c17dd8b8384bbc84b7e738
func ExtractHashFromRevision(rev string) Hash {
	if i := strings.LastIndex(rev, ":"); i != -1 {
		return Hash(rev[i+1:])
	}
	if ss := strings.Split(rev, "/"); len(ss) > 1 {
		return Hash(ss[len(ss)-1])
	}
	return Hash(rev)
}
