/*
Copyright 2025 The Flux authors

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

package storage_test

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/artifact/config"
	. "github.com/fluxcd/pkg/artifact/storage"
)

func TestStorage_LocalPath(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	opts := &config.Options{
		StoragePath:              tmpDir,
		StorageAddress:           ":9090",
		ArtifactRetentionTTL:     time.Minute,
		ArtifactRetentionRecords: 2,
	}
	s, err := New(opts)
	g.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name         string
		artifactPath string
		expectEmpty  bool
	}{
		{
			name:         "valid path",
			artifactPath: "gitrepository/default/test/source.tar.gz",
			expectEmpty:  false,
		},
		{
			name:         "path with subdirectories",
			artifactPath: "helmrepository/kube-system/charts/chart.tgz",
			expectEmpty:  false,
		},
		{
			name:         "empty path",
			artifactPath: "",
			expectEmpty:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			artifact := meta.Artifact{Path: tt.artifactPath}
			localPath := s.LocalPath(artifact)

			if tt.expectEmpty {
				g.Expect(localPath).To(BeEmpty())
			} else {
				g.Expect(localPath).To(HavePrefix(tmpDir))
				g.Expect(localPath).To(HaveSuffix(tt.artifactPath))
			}
		})
	}
}

func TestArtifactDir(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		namespace string
		objName   string
		expected  string
	}{
		{
			name:      "GitRepository",
			kind:      "GitRepository",
			namespace: "default",
			objName:   "test-repo",
			expected:  "gitrepository/default/test-repo",
		},
		{
			name:      "HelmRepository",
			kind:      "HelmRepository",
			namespace: "kube-system",
			objName:   "bitnami",
			expected:  "helmrepository/kube-system/bitnami",
		},
		{
			name:      "Bucket",
			kind:      "Bucket",
			namespace: "flux-system",
			objName:   "my-bucket",
			expected:  "bucket/flux-system/my-bucket",
		},
		{
			name:      "Mixed case kind",
			kind:      "MixedCaseKind",
			namespace: "test-ns",
			objName:   "test-obj",
			expected:  "mixedcasekind/test-ns/test-obj",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ArtifactDir(tt.kind, tt.namespace, tt.objName)
			if result != tt.expected {
				t.Errorf("ArtifactDir() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestArtifactPath(t *testing.T) {
	tests := []struct {
		name      string
		kind      string
		namespace string
		objName   string
		filename  string
		expected  string
	}{
		{
			name:      "GitRepository with tar.gz",
			kind:      "GitRepository",
			namespace: "default",
			objName:   "test-repo",
			filename:  "source.tar.gz",
			expected:  "gitrepository/default/test-repo/source.tar.gz",
		},
		{
			name:      "HelmRepository with tgz",
			kind:      "HelmRepository",
			namespace: "kube-system",
			objName:   "bitnami",
			filename:  "charts.tgz",
			expected:  "helmrepository/kube-system/bitnami/charts.tgz",
		},
		{
			name:      "Bucket with zip",
			kind:      "Bucket",
			namespace: "flux-system",
			objName:   "my-bucket",
			filename:  "data.zip",
			expected:  "bucket/flux-system/my-bucket/data.zip",
		},
		{
			name:      "Mixed case with special filename",
			kind:      "MixedCaseKind",
			namespace: "test-ns",
			objName:   "test-obj",
			filename:  "artifact-v1.0.0.tar.gz",
			expected:  "mixedcasekind/test-ns/test-obj/artifact-v1.0.0.tar.gz",
		},
		{
			name:      "Empty filename",
			kind:      "GitRepository",
			namespace: "default",
			objName:   "test-repo",
			filename:  "",
			expected:  "gitrepository/default/test-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ArtifactPath(tt.kind, tt.namespace, tt.objName, tt.filename)
			if result != tt.expected {
				t.Errorf("ArtifactPath() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
