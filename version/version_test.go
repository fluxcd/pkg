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
	"testing"

	. "github.com/onsi/gomega"

	"github.com/Masterminds/semver/v3"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		version string
		err     bool
	}{
		{"v1.2.3", false},
		{"v1.0", true},
		{"v1", true},
		{"v1.2.beta", true},
		{"v1.2-5", true},
		{"v1.2-beta.5", true},
		{"\nv1.2", true},
		{"v1.2.0-x.Y.0+metadata", false},
		{"v1.2.0-x.Y.0+metadata-width-hypen", false},
		{"v1.2.3-rc1-with-hypen", false},
		{"v1.2.3.4", true},
	}

	for _, tc := range tests {
		g := NewWithT(t)
		_, err := ParseVersion(tc.version)
		if tc.err {
			g.Expect(err).To(HaveOccurred())
		} else {
			g.Expect(err).NotTo(HaveOccurred())
		}
	}
}

func TestSort(t *testing.T) {
	g := NewWithT(t)

	constraint, err := semver.NewConstraint(">= 1.2.0, < 1.3.0")
	g.Expect(err).NotTo(HaveOccurred())

	sorted := Sort(constraint, []string{
		"v1.2.0",
		"v1.2.3",
		"v1.3.0",
		"1.2.4",
		"v1.1.0",
		"something-invalid",
		"v1.2.4",
		"another-invalid",
		"1.2.4",
	})

	g.Expect(sorted).To(Equal([]string{
		// Sort is stable.
		"1.2.4",
		"v1.2.4",
		"1.2.4",
		"v1.2.3",
		"v1.2.0",
	}))
}
