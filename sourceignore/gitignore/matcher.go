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

// This package was originally copied from the `go-git` project:
//
//	https://github.com/go-git/go-git/tree/v5.16.4/plumbing/format/gitignore
package gitignore

// Matcher defines a global multi-pattern matcher for gitignore patterns
type Matcher interface {
	// Match matches patterns in the order of priorities. As soon as an inclusion or
	// exclusion is found, not further matching is performed.
	Match(path []string, isDir bool) bool
}

// NewMatcher constructs a new global matcher. Patterns must be given in the order of
// increasing priority. That is most generic settings files first, then the content of
// the repo .gitignore, then content of .gitignore down the path or the repo and then
// the content command line arguments.
func NewMatcher(ps []Pattern) Matcher {
	return &matcher{ps}
}

type matcher struct {
	patterns []Pattern
}

func (m *matcher) Match(path []string, isDir bool) bool {
	n := len(m.patterns)
	for i := n - 1; i >= 0; i-- {
		if match := m.patterns[i].Match(path, isDir); match > NoMatch {
			return match == Exclude
		}
	}
	return false
}
