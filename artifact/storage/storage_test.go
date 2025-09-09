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
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/artifact/config"
	. "github.com/fluxcd/pkg/artifact/storage"
)

func TestStorageConstructor(t *testing.T) {
	dir := t.TempDir()

	opts := &config.Options{
		StoragePath:              "/nonexistent",
		StorageAddress:           ":9090",
		ArtifactRetentionTTL:     time.Minute,
		ArtifactRetentionRecords: 2,
	}
	if _, err := New(opts); err == nil {
		t.Fatal("nonexistent path was allowable in storage constructor")
	}

	f, err := os.CreateTemp(dir, "")
	if err != nil {
		t.Fatalf("while creating temporary file: %v", err)
	}
	f.Close()

	opts = &config.Options{
		StoragePath:              f.Name(),
		StorageAddress:           ":9090",
		ArtifactRetentionTTL:     time.Minute,
		ArtifactRetentionRecords: 2,
	}
	if _, err := New(opts); err == nil {
		os.Remove(f.Name())
		t.Fatal("file path was accepted as basedir")
	}
	os.Remove(f.Name())

	opts = &config.Options{
		StoragePath:              dir,
		StorageAddress:           ":9090",
		ArtifactRetentionTTL:     time.Minute,
		ArtifactRetentionRecords: 2,
	}
	if _, err := New(opts); err != nil {
		t.Fatalf("Valid path did not successfully return: %v", err)
	}
}

func TestStorage_NewArtifactFor(t *testing.T) {
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

	// Create a mock metadata object
	metadata := &mockMetadata{
		namespace: "test-namespace",
		name:      "test-resource",
	}

	artifact := s.NewArtifactFor("GitRepository", metadata, "main@sha1:abcd1234", "source.tar.gz")

	expectedPath := "gitrepository/test-namespace/test-resource/source.tar.gz"
	g.Expect(artifact.Path).To(Equal(expectedPath))
	g.Expect(artifact.Revision).To(Equal("main@sha1:abcd1234"))
	g.Expect(artifact.URL).To(Equal("http://localhost:9090/gitrepository/test-namespace/test-resource/source.tar.gz"))
}

func TestStorage_SetArtifactURL(t *testing.T) {
	tests := []struct {
		name         string
		hostname     string
		artifactPath string
		expectedURL  string
	}{
		{
			name:         "basic hostname",
			hostname:     "localhost:9090",
			artifactPath: "gitrepository/default/test/source.tar.gz",
			expectedURL:  "http://localhost:9090/gitrepository/default/test/source.tar.gz",
		},
		{
			name:         "hostname with http prefix",
			hostname:     "http://artifacts.example.com",
			artifactPath: "gitrepository/default/test/source.tar.gz",
			expectedURL:  "http://artifacts.example.com/gitrepository/default/test/source.tar.gz",
		},
		{
			name:         "hostname with https prefix",
			hostname:     "https://artifacts.example.com",
			artifactPath: "gitrepository/default/test/source.tar.gz",
			expectedURL:  "https://artifacts.example.com/gitrepository/default/test/source.tar.gz",
		},
		{
			name:         "path with leading slash",
			hostname:     "localhost:9090",
			artifactPath: "/gitrepository/default/test/source.tar.gz",
			expectedURL:  "http://localhost:9090/gitrepository/default/test/source.tar.gz",
		},
		{
			name:         "empty path",
			hostname:     "localhost:9090",
			artifactPath: "",
			expectedURL:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			s := &Storage{Hostname: tt.hostname}
			artifact := &meta.Artifact{Path: tt.artifactPath}

			s.SetArtifactURL(artifact)

			g.Expect(artifact.URL).To(Equal(tt.expectedURL))
		})
	}
}

func TestStorage_ArtifactExist(t *testing.T) {
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

	// Test with non-existent artifact
	artifact := meta.Artifact{Path: "gitrepository/default/test/nonexistent.tar.gz"}
	g.Expect(s.ArtifactExist(artifact)).To(BeFalse())

	// Create the artifact directory
	g.Expect(s.MkdirAll(artifact)).To(Succeed())

	// Test with directory instead of file
	dirArtifact := meta.Artifact{Path: "gitrepository/default/test"}
	g.Expect(s.ArtifactExist(dirArtifact)).To(BeFalse())

	// Create a real file
	artifactPath := s.LocalPath(artifact)
	g.Expect(os.WriteFile(artifactPath, []byte("test content"), 0600)).To(Succeed())

	// Test with existing regular file
	g.Expect(s.ArtifactExist(artifact)).To(BeTrue())

	// Create a symlink
	symlinkPath := filepath.Join(tmpDir, "gitrepository", "default", "test", "symlink.tar.gz")
	g.Expect(os.Symlink(artifactPath, symlinkPath)).To(Succeed())

	symlinkArtifact := meta.Artifact{Path: "gitrepository/default/test/symlink.tar.gz"}
	// ArtifactExist should return false for symlinks (only regular files)
	g.Expect(s.ArtifactExist(symlinkArtifact)).To(BeFalse())
}

func TestStorage_VerifyArtifact(t *testing.T) {
	g := NewWithT(t)

	dir := t.TempDir()
	opts := &config.Options{
		StoragePath:              dir,
		StorageAddress:           ":9090",
		ArtifactRetentionTTL:     0,
		ArtifactRetentionRecords: 0,
	}
	s, err := New(opts)
	g.Expect(err).ToNot(HaveOccurred(), "failed to create new storage")

	g.Expect(os.WriteFile(filepath.Join(dir, "artifact"), []byte("test"), 0o600)).To(Succeed())

	t.Run("artifact without digest", func(t *testing.T) {
		g := NewWithT(t)

		err := s.VerifyArtifact(meta.Artifact{})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err).To(MatchError("artifact has no digest"))
	})

	t.Run("artifact with invalid digest", func(t *testing.T) {
		g := NewWithT(t)

		err := s.VerifyArtifact(meta.Artifact{Digest: "invalid"})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err).To(MatchError("failed to parse artifact digest 'invalid': invalid checksum digest format"))
	})

	t.Run("artifact with invalid path", func(t *testing.T) {
		g := NewWithT(t)

		err := s.VerifyArtifact(meta.Artifact{
			Digest: "sha256:9ba7a35ce8acd3557fe30680ef193ca7a36bb5dc62788f30de7122a0a5beab69",
			Path:   "invalid",
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(os.IsNotExist(err)).To(BeTrue())
	})

	t.Run("artifact with digest mismatch", func(t *testing.T) {
		g := NewWithT(t)

		err := s.VerifyArtifact(meta.Artifact{
			Digest: "sha256:9ba7a35ce8acd3557fe30680ef193ca7a36bb5dc62788f30de7122a0a5beab69",
			Path:   "artifact",
		})
		g.Expect(err).To(HaveOccurred())
		g.Expect(err).To(MatchError("computed digest doesn't match 'sha256:9ba7a35ce8acd3557fe30680ef193ca7a36bb5dc62788f30de7122a0a5beab69'"))
	})

	t.Run("artifact with digest match", func(t *testing.T) {
		g := NewWithT(t)

		err := s.VerifyArtifact(meta.Artifact{
			Digest: "sha256:9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
			Path:   "artifact",
		})
		g.Expect(err).ToNot(HaveOccurred())
	})
}

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyz1234567890")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

// mockMetadata implements metav1.Object for testing
type mockMetadata struct {
	namespace string
	name      string
}

func (m *mockMetadata) GetNamespace() string                          { return m.namespace }
func (m *mockMetadata) GetName() string                               { return m.name }
func (m *mockMetadata) GetGenerateName() string                       { return "" }
func (m *mockMetadata) SetGenerateName(string)                        {}
func (m *mockMetadata) GetUID() types.UID                             { return "" }
func (m *mockMetadata) SetUID(types.UID)                              {}
func (m *mockMetadata) GetResourceVersion() string                    { return "" }
func (m *mockMetadata) SetResourceVersion(string)                     {}
func (m *mockMetadata) GetGeneration() int64                          { return 0 }
func (m *mockMetadata) SetGeneration(int64)                           {}
func (m *mockMetadata) GetCreationTimestamp() metav1.Time             { return metav1.Time{} }
func (m *mockMetadata) SetCreationTimestamp(metav1.Time)              {}
func (m *mockMetadata) GetDeletionTimestamp() *metav1.Time            { return nil }
func (m *mockMetadata) SetDeletionTimestamp(*metav1.Time)             {}
func (m *mockMetadata) GetDeletionGracePeriodSeconds() *int64         { return nil }
func (m *mockMetadata) SetDeletionGracePeriodSeconds(*int64)          {}
func (m *mockMetadata) GetLabels() map[string]string                  { return nil }
func (m *mockMetadata) SetLabels(map[string]string)                   {}
func (m *mockMetadata) GetAnnotations() map[string]string             { return nil }
func (m *mockMetadata) SetAnnotations(map[string]string)              {}
func (m *mockMetadata) GetFinalizers() []string                       { return nil }
func (m *mockMetadata) SetFinalizers([]string)                        {}
func (m *mockMetadata) SetNamespace(string)                           {}
func (m *mockMetadata) SetName(string)                                {}
func (m *mockMetadata) GetManagedFields() []metav1.ManagedFieldsEntry { return nil }
func (m *mockMetadata) SetManagedFields([]metav1.ManagedFieldsEntry)  {}
func (m *mockMetadata) GetOwnerReferences() []metav1.OwnerReference   { return nil }
func (m *mockMetadata) SetOwnerReferences([]metav1.OwnerReference)    {}
func (m *mockMetadata) GetSelfLink() string                           { return "" }
func (m *mockMetadata) SetSelfLink(string)                            {}
