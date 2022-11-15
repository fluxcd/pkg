/*
Copyright 2020, 2021 The Flux authors

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

package libgit2

import (
	"errors"
	"strings"
)

// LibGit2Error translates an error from the libgit2 library, or
// returns `nil` if the argument is `nil`.
func LibGit2Error(err error) error {
	if err == nil {
		return err
	}
	// libgit2 returns the whole output from stderr, and we only need
	// the message. GitLab likes to return a banner, so as an
	// heuristic, strip any lines that are just "remote:" and spaces
	// or fencing.
	msg := err.Error()
	lines := strings.Split(msg, "\n")
	if len(lines) == 1 {
		return err
	}
	var b strings.Builder
	// the following removes the prefix "remote:" from each line; to
	// retain a bit of fidelity to the original error, start with it.
	b.WriteString("remote: ")

	var appending bool
	for _, line := range lines {
		m := strings.TrimPrefix(line, "remote:")
		if m = strings.Trim(m, " \t="); m != "" {
			if appending {
				b.WriteString(" ")
			}
			b.WriteString(m)
			appending = true
		}
	}
	return errors.New(b.String())
}
