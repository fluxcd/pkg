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
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/artifact/config"
	. "github.com/fluxcd/pkg/artifact/storage"
)

func TestStorage_CopyFromPath(t *testing.T) {
	type File struct {
		Name    string
		Content []byte
	}

	dir := t.TempDir()

	opts := &config.Options{
		StoragePath:              dir,
		StorageAddress:           ":9090",
		ArtifactRetentionTTL:     time.Minute,
		ArtifactRetentionRecords: 2,
	}
	storage, err := New(opts)
	if err != nil {
		t.Fatalf("error while bootstrapping storage: %v", err)
	}

	createFile := func(file *File) (absPath string, err error) {
		dir = t.TempDir()
		absPath = filepath.Join(dir, file.Name)
		if err = os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
			return
		}
		f, err := os.Create(absPath)
		if err != nil {
			return "", fmt.Errorf("could not create file %q: %w", absPath, err)
		}
		if n, err := f.Write(file.Content); err != nil {
			f.Close()
			return "", fmt.Errorf("could not write %d bytes to file %q: %w", n, f.Name(), err)
		}
		f.Close()
		return
	}

	matchFile := func(t *testing.T, storage *Storage, artifact meta.Artifact, file *File, expectMismatch bool) {
		c, err := os.ReadFile(storage.LocalPath(artifact))
		if err != nil {
			t.Fatalf("failed reading file: %v", err)
		}
		if (string(c) != string(file.Content)) != expectMismatch {
			t.Errorf("artifact content does not match and not expecting mismatch, got: %q, want: %q", string(c), string(file.Content))
		}
	}

	tests := []struct {
		name           string
		file           *File
		want           *File
		expectMismatch bool
	}{
		{
			name: "content match",
			file: &File{
				Name:    "manifest.yaml",
				Content: []byte(`contents`),
			},
			want: &File{
				Name:    "manifest.yaml",
				Content: []byte(`contents`),
			},
		},
		{
			name: "content not match",
			file: &File{
				Name:    "manifest.yaml",
				Content: []byte(`contents`),
			},
			want: &File{
				Name:    "manifest.yaml",
				Content: []byte(`mismatch contents`),
			},
			expectMismatch: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absPath, err := createFile(tt.file)
			if err != nil {
				t.Error(err)
				return
			}
			artifact := meta.Artifact{
				Path: filepath.Join(randStringRunes(10), randStringRunes(10), randStringRunes(10)),
			}
			if err := storage.MkdirAll(artifact); err != nil {
				t.Fatalf("artifact directory creation failed: %v", err)
			}
			if err := storage.CopyFromPath(&artifact, absPath); err != nil {
				t.Errorf("CopyFromPath() error = %v", err)
			}
			matchFile(t, storage, artifact, tt.want, tt.expectMismatch)
		})
	}
}

func TestStorage_CopyToPath(t *testing.T) {
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

	// Create a test directory structure to archive
	sourceDir := t.TempDir()
	testFiles := map[string]string{
		"README.md":           "# Test Repository\n",
		"src/main.go":         "package main\n\nfunc main() {}\n",
		"src/utils/helper.go": "package utils\n\nfunc Help() {}\n",
		"config/app.yaml":     "name: test-app\nversion: 1.0.0\n",
	}

	// Create the test files
	for filePath, content := range testFiles {
		fullPath := filepath.Join(sourceDir, filePath)
		g.Expect(os.MkdirAll(filepath.Dir(fullPath), 0755)).To(Succeed())
		g.Expect(os.WriteFile(fullPath, []byte(content), 0644)).To(Succeed())
	}

	// Create an artifact and archive the source directory
	artifact := meta.Artifact{
		Path: "gitrepository/default/test-repo/source.tar.gz",
	}
	g.Expect(s.MkdirAll(artifact)).To(Succeed())
	g.Expect(s.Archive(&artifact, sourceDir, nil)).To(Succeed())

	tests := []struct {
		name        string
		subPath     string
		expectFiles map[string]string
		expectError bool
	}{
		{
			name:    "copy entire archive",
			subPath: "",
			expectFiles: map[string]string{
				"README.md":           "# Test Repository\n",
				"src/main.go":         "package main\n\nfunc main() {}\n",
				"src/utils/helper.go": "package utils\n\nfunc Help() {}\n",
				"config/app.yaml":     "name: test-app\nversion: 1.0.0\n",
			},
		},
		{
			name:    "copy specific directory",
			subPath: "src",
			expectFiles: map[string]string{
				"main.go":         "package main\n\nfunc main() {}\n",
				"utils/helper.go": "package utils\n\nfunc Help() {}\n",
			},
		},
		{
			name:    "copy specific file",
			subPath: "README.md",
			expectFiles: map[string]string{
				"": "# Test Repository\n", // Single file copied directly
			},
		},
		{
			name:    "copy subdirectory",
			subPath: "src/utils",
			expectFiles: map[string]string{
				"helper.go": "package utils\n\nfunc Help() {}\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create destination directory
			destDir := t.TempDir()
			toPath := filepath.Join(destDir, "extracted")

			err := s.CopyToPath(&artifact, tt.subPath, toPath)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).NotTo(HaveOccurred())

			// Verify extracted content
			for expectedFile, expectedContent := range tt.expectFiles {
				var checkPath string
				if expectedFile == "" {
					// Single file case
					checkPath = toPath
				} else {
					checkPath = filepath.Join(toPath, expectedFile)
				}

				g.Expect(checkPath).To(BeAnExistingFile())
				content, err := os.ReadFile(checkPath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(content)).To(Equal(expectedContent))
			}
		})
	}
}

func TestStorage_CopyToPath_Errors(t *testing.T) {
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

	t.Run("non-existent artifact", func(t *testing.T) {
		g := NewWithT(t)

		artifact := meta.Artifact{
			Path: "gitrepository/default/nonexistent/source.tar.gz",
		}

		destPath := filepath.Join(t.TempDir(), "dest")
		err := s.CopyToPath(&artifact, "", destPath)
		g.Expect(err).To(HaveOccurred())
	})

	t.Run("invalid subpath", func(t *testing.T) {
		g := NewWithT(t)

		// Create a simple artifact
		sourceDir := t.TempDir()
		g.Expect(os.WriteFile(filepath.Join(sourceDir, "test.txt"), []byte("test"), 0644)).To(Succeed())

		artifact := meta.Artifact{
			Path: "gitrepository/default/test/source.tar.gz",
		}
		g.Expect(s.MkdirAll(artifact)).To(Succeed())
		g.Expect(s.Archive(&artifact, sourceDir, nil)).To(Succeed())

		destPath := filepath.Join(t.TempDir(), "dest")
		err := s.CopyToPath(&artifact, "nonexistent/path", destPath)
		g.Expect(err).To(HaveOccurred())
	})
}

func TestStorage_Remove(t *testing.T) {
	t.Run("removes file", func(t *testing.T) {
		g := NewWithT(t)

		dir := t.TempDir()

		opts := &config.Options{
			StoragePath:              dir,
			StorageAddress:           ":9090",
			ArtifactRetentionTTL:     0,
			ArtifactRetentionRecords: 0,
		}
		s, err := New(opts)
		g.Expect(err).ToNot(HaveOccurred())

		artifact := meta.Artifact{
			Path: filepath.Join(dir, "test.txt"),
		}
		g.Expect(s.MkdirAll(artifact)).To(Succeed())
		g.Expect(s.AtomicWriteFile(&artifact, bytes.NewReader([]byte("test")), 0o600)).To(Succeed())
		g.Expect(s.ArtifactExist(artifact)).To(BeTrue())

		g.Expect(s.Remove(artifact)).To(Succeed())
		g.Expect(s.ArtifactExist(artifact)).To(BeFalse())
	})

	t.Run("error if file does not exist", func(t *testing.T) {
		g := NewWithT(t)

		dir := t.TempDir()

		opts := &config.Options{
			StoragePath:              dir,
			StorageAddress:           ":9090",
			ArtifactRetentionTTL:     0,
			ArtifactRetentionRecords: 0,
		}
		s, err := New(opts)
		g.Expect(err).ToNot(HaveOccurred())

		artifact := meta.Artifact{
			Path: filepath.Join(dir, "test.txt"),
		}

		err = s.Remove(artifact)
		g.Expect(err).To(HaveOccurred())
		g.Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
	})
}

func TestStorage_RemoveAllButCurrent(t *testing.T) {
	t.Run("bad directory in archive", func(t *testing.T) {
		dir := t.TempDir()

		opts := &config.Options{
			StoragePath:              dir,
			StorageAddress:           ":9090",
			ArtifactRetentionTTL:     time.Minute,
			ArtifactRetentionRecords: 2,
		}
		s, err := New(opts)
		if err != nil {
			t.Fatalf("Valid path did not successfully return: %v", err)
		}

		if _, err := s.RemoveAllButCurrent(meta.Artifact{Path: filepath.Join(dir, "really", "nonexistent")}); err == nil {
			t.Fatal("Did not error while pruning non-existent path")
		}
	})

	t.Run("collect names of deleted items", func(t *testing.T) {
		g := NewWithT(t)
		dir := t.TempDir()

		opts := &config.Options{
			StoragePath:              dir,
			StorageAddress:           ":9090",
			ArtifactRetentionTTL:     time.Minute,
			ArtifactRetentionRecords: 2,
		}
		s, err := New(opts)
		g.Expect(err).ToNot(HaveOccurred(), "failed to create new storage")

		artifact := meta.Artifact{
			Path: filepath.Join("foo", "bar", "artifact1.tar.gz"),
		}

		// Create artifact dir and artifacts.
		artifactDir := filepath.Join(dir, "foo", "bar")
		g.Expect(os.MkdirAll(artifactDir, 0o750)).NotTo(HaveOccurred())
		current := []string{
			filepath.Join(artifactDir, "artifact1.tar.gz"),
		}
		wantDeleted := []string{
			filepath.Join(artifactDir, "file1.txt"),
			filepath.Join(artifactDir, "file2.txt"),
		}
		createFile := func(files []string) {
			for _, c := range files {
				f, err := os.Create(c)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(f.Close()).ToNot(HaveOccurred())
			}
		}
		createFile(current)
		createFile(wantDeleted)
		_, err = s.Symlink(artifact, "latest.tar.gz")
		g.Expect(err).ToNot(HaveOccurred(), "failed to create symlink")

		deleted, err := s.RemoveAllButCurrent(artifact)
		g.Expect(err).ToNot(HaveOccurred(), "failed to remove all but current")
		g.Expect(deleted).To(Equal(wantDeleted))
	})
}

func TestStorage_RemoveAll(t *testing.T) {
	tests := []struct {
		name               string
		artifactPath       string
		createArtifactPath bool
		wantDeleted        string
	}{
		{
			name:               "delete non-existent path",
			artifactPath:       filepath.Join("foo", "bar", "artifact1.tar.gz"),
			createArtifactPath: false,
			wantDeleted:        "",
		},
		{
			name:               "delete existing path",
			artifactPath:       filepath.Join("foo", "bar", "artifact1.tar.gz"),
			createArtifactPath: true,
			wantDeleted:        filepath.Join("foo", "bar"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()

			opts := &config.Options{
				StoragePath:              dir,
				StorageAddress:           ":9090",
				ArtifactRetentionTTL:     time.Minute,
				ArtifactRetentionRecords: 2,
			}
			s, err := New(opts)
			g.Expect(err).ToNot(HaveOccurred(), "failed to create new storage")

			artifact := meta.Artifact{
				Path: tt.artifactPath,
			}

			if tt.createArtifactPath {
				g.Expect(os.MkdirAll(filepath.Join(dir, tt.artifactPath), 0o750)).ToNot(HaveOccurred())
			}

			deleted, err := s.RemoveAll(artifact)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(deleted).To(ContainSubstring(tt.wantDeleted), "unexpected deleted path")
		})
	}
}
