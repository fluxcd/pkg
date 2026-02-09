/*
Copyright 2023 The Flux authors

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

package kustomize

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/sourceignore"
	"github.com/fluxcd/pkg/sourceignore/gitignore"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func TestScanManifests(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		base      string
		wantErr   bool
		wantPaths []string
	}{
		{
			name: "empty directory",
			base: tmpDir,
		},
		{
			name: "valid manifests",
			base: "./testdata/nokustomization/resources",
			wantPaths: []string{
				"testdata/nokustomization/resources/configmap.yaml",
				"testdata/nokustomization/resources/secret.yaml",
			},
		},
		{
			name:    "malformed YAML - panic recovery error",
			base:    "./testdata/nokustomization/panic",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fs := filesys.MakeFsOnDisk()

			paths, err := scanManifests(fs, tt.base, nil, nil)
			g.Expect(paths).To(Equal(tt.wantPaths))
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}

func TestScanManifests_WithIgnorePatterns(t *testing.T) {
	tests := []struct {
		name           string
		base           string
		ignorePatterns string
		wantPaths      []string
		wantErr        bool
	}{
		{
			name:      "basic directory - no ignore patterns",
			base:      "./testdata/ignore-tests/basic",
			wantPaths: []string{"testdata/ignore-tests/basic/deployment.yaml"},
		},
		{
			name:           "with .sops.yaml - no ignore patterns (should fail)",
			base:           "./testdata/ignore-tests/with-sops",
			ignorePatterns: "",
			wantErr:        true, // .sops.yaml should cause parsing error
		},
		{
			name:           "with .sops.yaml - ignore .sops.yaml",
			base:           "./testdata/ignore-tests/with-sops",
			ignorePatterns: ".sops.yaml",
			wantPaths:      []string{"testdata/ignore-tests/with-sops/deployment.yaml"},
		},
		{
			name:           "with .gitlab-ci.yml - no ignore patterns (should fail)",
			base:           "./testdata/ignore-tests/with-gitlab-ci",
			ignorePatterns: "",
			wantErr:        true, // .gitlab-ci.yml should cause parsing error
		},
		{
			name:           "with .gitlab-ci.yml - ignore .gitlab-ci.yml",
			base:           "./testdata/ignore-tests/with-gitlab-ci",
			ignorePatterns: ".gitlab-ci.yml",
			wantPaths:      []string{"testdata/ignore-tests/with-gitlab-ci/deployment.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fs := filesys.MakeFsOnDisk()

			// Convert string patterns to the expected format
			var ignorePatterns []gitignore.Pattern
			var ignoreDomain []string

			if tt.ignorePatterns != "" {
				absBase, err := filepath.Abs(tt.base)
				g.Expect(err).NotTo(HaveOccurred())

				ignoreDomain = strings.Split(absBase, string(filepath.Separator))

				// Load existing patterns from ignore files
				ignorePatterns, err = sourceignore.LoadIgnorePatterns(absBase, ignoreDomain)
				g.Expect(err).NotTo(HaveOccurred())

				// Add the test-specific patterns
				ignorePatterns = append(ignorePatterns,
					sourceignore.ReadPatterns(strings.NewReader(tt.ignorePatterns), ignoreDomain)...)
			}

			paths, err := scanManifests(fs, tt.base, ignorePatterns, ignoreDomain)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())
			sort.Strings(paths)
			sort.Strings(tt.wantPaths)
			g.Expect(paths).To(Equal(tt.wantPaths))
		})
	}
}
