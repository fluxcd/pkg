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
	"testing"

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
		{
			name: "utf-16LE with BOM files - should be valid",
			base: "./testdata/nokustomization/utf16le",
			wantPaths: []string{
				"testdata/nokustomization/utf16le/configmap.yaml",
				"testdata/nokustomization/utf16le/secret.yaml",
			},
		},
		{
			name:    "utf-16LE without BOM files - should be invalid",
			base:    "./testdata/nokustomization/utf16le-no-bom",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			fs := filesys.MakeFsOnDisk()

			paths, err := scanManifests(fs, tt.base)
			g.Expect(paths).To(Equal(tt.wantPaths))
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}
