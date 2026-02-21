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
		{"v2025.07.03", false},
		{"v1.0", true},
		{"v1", true},
		{"v1.2.beta", true},
		{"v1.2-5", true},
		{"v1.2-beta5", true},
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
			g.Expect(err).To(HaveOccurred(), "version: %s", tc.version)
		} else {
			g.Expect(err).NotTo(HaveOccurred(), "version: %s", tc.version)
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

func TestRepoMajor(t *testing.T) {
	g := NewWithT(t)
	g.Expect(RepoMajor("flux2")).To(Equal(2))
	g.Expect(RepoMajor("source-controller")).To(Equal(1))
	g.Expect(RepoMajor("kustomize-controller")).To(Equal(1))
	g.Expect(RepoMajor("helm-controller")).To(Equal(1))
	g.Expect(RepoMajor("notification-controller")).To(Equal(1))
	g.Expect(RepoMajor("image-reflector-controller")).To(Equal(1))
	g.Expect(RepoMajor("image-automation-controller")).To(Equal(1))
	g.Expect(RepoMajor("source-watcher")).To(Equal(2))

	_, err := RepoMajor("unknown-controller")
	g.Expect(err).To(HaveOccurred())
}

func TestRepoMinorForFluxMinor(t *testing.T) {
	tests := []struct {
		name       string
		repoName   string
		flux2Minor int
		expected   int
		expectErr  bool
	}{
		{
			name:       "flux2 identity",
			repoName:   "flux2",
			flux2Minor: 7,
			expected:   7,
		},
		{
			name:       "flux2 higher minor",
			repoName:   "flux2",
			flux2Minor: 10,
			expected:   10,
		},
		{
			name:       "source-controller at baseline",
			repoName:   "source-controller",
			flux2Minor: 7,
			expected:   7,
		},
		{
			name:       "source-controller above baseline",
			repoName:   "source-controller",
			flux2Minor: 9,
			expected:   9,
		},
		{
			name:       "helm-controller at baseline",
			repoName:   "helm-controller",
			flux2Minor: 7,
			expected:   4,
		},
		{
			name:       "helm-controller above baseline",
			repoName:   "helm-controller",
			flux2Minor: 10,
			expected:   7,
		},
		{
			name:       "image-reflector-controller at baseline",
			repoName:   "image-reflector-controller",
			flux2Minor: 7,
			expected:   0,
		},
		{
			name:       "image-reflector-controller above baseline",
			repoName:   "image-reflector-controller",
			flux2Minor: 8,
			expected:   1,
		},
		{
			name:       "source-watcher at baseline",
			repoName:   "source-watcher",
			flux2Minor: 7,
			expected:   0,
		},
		{
			// Negative results are valid: the baseline is a reference
			// point for the linear offset, not a statement about when
			// the repository was introduced in the distribution. We
			// don't offer such API in this package at the moment.
			name:       "source-watcher before baseline",
			repoName:   "source-watcher",
			flux2Minor: 6,
			expected:   -1,
		},
		{
			name:       "unknown repository",
			repoName:   "unknown-repo",
			flux2Minor: 7,
			expectErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result, err := RepoMinorForFluxMinor(tc.repoName, tc.flux2Minor)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(tc.expected))
			}
		})
	}
}

func TestFluxMinorForRepoMinor(t *testing.T) {
	tests := []struct {
		name      string
		repoName  string
		repoMinor int
		expected  int
		expectErr bool
	}{
		{
			name:      "flux2 identity",
			repoName:  "flux2",
			repoMinor: 7,
			expected:  7,
		},
		{
			name:      "flux2 higher minor",
			repoName:  "flux2",
			repoMinor: 10,
			expected:  10,
		},
		{
			name:      "source-controller at baseline",
			repoName:  "source-controller",
			repoMinor: 7,
			expected:  7,
		},
		{
			name:      "source-controller above baseline",
			repoName:  "source-controller",
			repoMinor: 9,
			expected:  9,
		},
		{
			name:      "helm-controller at baseline",
			repoName:  "helm-controller",
			repoMinor: 4,
			expected:  7,
		},
		{
			name:      "helm-controller above baseline",
			repoName:  "helm-controller",
			repoMinor: 7,
			expected:  10,
		},
		{
			name:      "image-reflector-controller at baseline",
			repoName:  "image-reflector-controller",
			repoMinor: 0,
			expected:  7,
		},
		{
			name:      "image-reflector-controller above baseline",
			repoName:  "image-reflector-controller",
			repoMinor: 1,
			expected:  8,
		},
		{
			name:      "source-watcher at baseline",
			repoName:  "source-watcher",
			repoMinor: 0,
			expected:  7,
		},
		{
			name:      "helm-controller before baseline",
			repoName:  "helm-controller",
			repoMinor: 3,
			expected:  6,
		},
		{
			name:      "unknown repository",
			repoName:  "unknown-repo",
			repoMinor: 7,
			expectErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result, err := FluxMinorForRepoMinor(tc.repoName, tc.repoMinor)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(tc.expected))
			}
		})
	}
}

func TestRepoMinorFluxMinorRoundTrip(t *testing.T) {
	g := NewWithT(t)

	for repoName, cv := range baseline {
		for flux2Minor := cv.flux2Minor; flux2Minor <= cv.flux2Minor+5; flux2Minor++ {
			repoMinor, err := RepoMinorForFluxMinor(repoName, flux2Minor)
			g.Expect(err).NotTo(HaveOccurred(), "repo: %s, flux2Minor: %d", repoName, flux2Minor)

			roundTripped, err := FluxMinorForRepoMinor(repoName, repoMinor)
			g.Expect(err).NotTo(HaveOccurred(), "repo: %s, repoMinor: %d", repoName, repoMinor)
			g.Expect(roundTripped).To(Equal(flux2Minor), "round-trip failed for repo: %s, flux2Minor: %d", repoName, flux2Minor)
		}
	}
}
