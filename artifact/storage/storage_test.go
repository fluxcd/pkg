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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
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

func TestStorage_Archive(t *testing.T) {
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

	type dummyFile struct {
		content []byte
		mode    int64
	}

	createFiles := func(files map[string]dummyFile) (dir string, err error) {
		dir = t.TempDir()
		for name, df := range files {
			absPath := filepath.Join(dir, name)
			if err = os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
				return
			}
			f, err := os.Create(absPath)
			if err != nil {
				return "", fmt.Errorf("could not create file %q: %w", absPath, err)
			}
			if n, err := f.Write(df.content); err != nil {
				f.Close()
				return "", fmt.Errorf("could not write %d bytes to file %q: %w", n, f.Name(), err)
			}
			f.Close()

			if df.mode != 0 {
				if err = os.Chmod(absPath, os.FileMode(df.mode)); err != nil {
					return "", fmt.Errorf("could not chmod file %q: %w", absPath, err)
				}
			}
		}
		return
	}

	matchFiles := func(t *testing.T, storage *Storage, artifact meta.Artifact, files map[string]dummyFile, dirs []string) {
		t.Helper()
		for name, df := range files {
			mustExist := !(name[0:1] == "!")
			if !mustExist {
				name = name[1:]
			}
			s, m, exist, err := walkTar(storage.LocalPath(artifact), name, false)
			if err != nil {
				t.Fatalf("failed reading tarball: %v", err)
			}
			if bs := int64(len(df.content)); s != bs {
				t.Fatalf("%q size %v != %v", name, s, bs)
			}
			if exist != mustExist {
				if mustExist {
					t.Errorf("could not find file %q in tarball", name)
				} else {
					t.Errorf("tarball contained excluded file %q", name)
				}
			}
			expectMode := df.mode
			if expectMode == 0 {
				expectMode = DefaultFileMode
			}
			if exist && m != expectMode {
				t.Fatalf("%q mode %v != %v", name, m, expectMode)
			}
		}
		for _, name := range dirs {
			mustExist := !(name[0:1] == "!")
			if !mustExist {
				name = name[1:]
			}
			_, m, exist, err := walkTar(storage.LocalPath(artifact), name, true)
			if err != nil {
				t.Fatalf("failed reading tarball: %v", err)
			}
			if exist != mustExist {
				if mustExist {
					t.Errorf("could not find dir %q in tarball", name)
				} else {
					t.Errorf("tarball contained excluded file %q", name)
				}
			}
			if exist && m != DefaultDirMode {
				t.Fatalf("%q mode %v != %v", name, m, DefaultDirMode)
			}

		}
	}

	tests := []struct {
		name     string
		files    map[string]dummyFile
		filter   ArchiveFileFilter
		want     map[string]dummyFile
		wantDirs []string
		wantErr  bool
	}{
		{
			name: "no filter",
			files: map[string]dummyFile{
				".git/config":   {},
				"file.jpg":      {content: []byte(`contents`)},
				"manifest.yaml": {},
			},
			filter: nil,
			want: map[string]dummyFile{
				".git/config":   {},
				"file.jpg":      {content: []byte(`contents`)},
				"manifest.yaml": {},
			},
		},
		{
			name: "exclude VCS",
			files: map[string]dummyFile{
				".git/config":   {},
				"manifest.yaml": {},
			},
			wantDirs: []string{
				"!.git",
			},
			filter: SourceIgnoreFilter(nil, nil),
			want: map[string]dummyFile{
				"!.git/config":  {},
				"manifest.yaml": {},
			},
		},
		{
			name: "custom",
			files: map[string]dummyFile{
				".git/config": {},
				"custom":      {},
				"horse.jpg":   {},
			},
			filter: SourceIgnoreFilter([]gitignore.Pattern{
				gitignore.ParsePattern("custom", nil),
			}, nil),
			want: map[string]dummyFile{
				"!git/config": {},
				"!custom":     {},
				"horse.jpg":   {},
			},
			wantErr: false,
		},
		{
			name: "including directories",
			files: map[string]dummyFile{
				"test/.gitkeep": {},
			},
			filter: SourceIgnoreFilter([]gitignore.Pattern{
				gitignore.ParsePattern("custom", nil),
			}, nil),
			wantDirs: []string{
				"test",
			},
			wantErr: false,
		},
		{
			name: "sets default file modes",
			files: map[string]dummyFile{
				"test/file": {
					mode: 0o666,
				},
				"test/executable": {
					mode: 0o777,
				},
			},
			want: map[string]dummyFile{
				"test/file": {
					mode: DefaultFileMode,
				},
				"test/executable": {
					mode: DefaultExeFileMode,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, err := createFiles(tt.files)
			if err != nil {
				t.Error(err)
				return
			}
			defer os.RemoveAll(dir)
			artifact := meta.Artifact{
				Path: filepath.Join(randStringRunes(10), randStringRunes(10), randStringRunes(10)+".tar.gz"),
			}
			if err := storage.MkdirAll(artifact); err != nil {
				t.Fatalf("artifact directory creation failed: %v", err)
			}
			if err := storage.Archive(&artifact, dir, tt.filter); (err != nil) != tt.wantErr {
				t.Errorf("Archive() error = %v, wantErr %v", err, tt.wantErr)
			}
			matchFiles(t, storage, artifact, tt.want, tt.wantDirs)
		})
	}
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

func TestStorageRemoveAllButCurrent(t *testing.T) {
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

func TestStorageRemoveAll(t *testing.T) {
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

func TestStorageCopyFromPath(t *testing.T) {
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

func TestStorage_getGarbageFiles(t *testing.T) {
	artifactFolder := filepath.Join("foo", "bar")
	tests := []struct {
		name                 string
		artifactPaths        []string
		createPause          time.Duration
		ttl                  time.Duration
		maxItemsToBeRetained int
		totalCountLimit      int
		wantDeleted          []string
	}{
		{
			name: "delete files based on maxItemsToBeRetained",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
			},
			createPause:          time.Millisecond * 10,
			ttl:                  time.Minute * 2,
			totalCountLimit:      10,
			maxItemsToBeRetained: 2,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
			},
		},
		{
			name: "delete files based on maxItemsToBeRetained, ignore lock files",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact1.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
			},
			createPause:          time.Millisecond * 10,
			ttl:                  time.Minute * 2,
			totalCountLimit:      10,
			maxItemsToBeRetained: 2,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
			},
		},
		{
			name: "delete files based on ttl",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
			},
			createPause:          time.Second * 1,
			ttl:                  time.Second*3 + time.Millisecond*500,
			totalCountLimit:      10,
			maxItemsToBeRetained: 4,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
			},
		},
		{
			name: "delete files based on ttl, ignore lock files",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact1.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
			},
			createPause:          time.Second * 1,
			ttl:                  time.Second*3 + time.Millisecond*500,
			totalCountLimit:      10,
			maxItemsToBeRetained: 4,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
			},
		},
		{
			name: "delete files based on ttl and maxItemsToBeRetained",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
				filepath.Join(artifactFolder, "artifact6.tar.gz"),
			},
			createPause:          time.Second * 1,
			ttl:                  time.Second*5 + time.Millisecond*500,
			totalCountLimit:      10,
			maxItemsToBeRetained: 4,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
			},
		},
		{
			name: "delete files based on ttl and maxItemsToBeRetained and totalCountLimit",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
				filepath.Join(artifactFolder, "artifact5.tar.gz"),
				filepath.Join(artifactFolder, "artifact6.tar.gz"),
			},
			createPause:          time.Millisecond * 500,
			ttl:                  time.Millisecond * 500,
			totalCountLimit:      3,
			maxItemsToBeRetained: 2,
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()

			opts := &config.Options{
				StoragePath:              dir,
				StorageAddress:           ":9090",
				ArtifactRetentionTTL:     tt.ttl,
				ArtifactRetentionRecords: tt.maxItemsToBeRetained,
			}
			s, err := New(opts)
			g.Expect(err).ToNot(HaveOccurred(), "failed to create new storage")

			artifact := meta.Artifact{
				Path: tt.artifactPaths[len(tt.artifactPaths)-1],
			}
			g.Expect(os.MkdirAll(filepath.Join(dir, artifactFolder), 0o750)).ToNot(HaveOccurred())
			for _, artifactPath := range tt.artifactPaths {
				f, err := os.Create(filepath.Join(dir, artifactPath))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(f.Close()).ToNot(HaveOccurred())
				time.Sleep(tt.createPause)
			}

			deletedPaths, err := s.GetGarbageFiles(artifact, tt.totalCountLimit, tt.maxItemsToBeRetained, tt.ttl)
			g.Expect(err).ToNot(HaveOccurred(), "failed to collect garbage files")
			g.Expect(len(tt.wantDeleted)).To(Equal(len(deletedPaths)))
			for _, wantDeletedPath := range tt.wantDeleted {
				present := false
				for _, deletedPath := range deletedPaths {
					if strings.Contains(deletedPath, wantDeletedPath) {
						present = true
						break
					}
				}
				if !present {
					g.Fail(fmt.Sprintf("expected file to be deleted, still exists: %s", wantDeletedPath))
				}
			}
		})
	}
}

func TestStorage_GarbageCollect(t *testing.T) {
	artifactFolder := filepath.Join("foo", "bar")
	tests := []struct {
		name          string
		artifactPaths []string
		wantCollected []string
		wantDeleted   []string
		wantErr       string
		ctxTimeout    time.Duration
	}{
		{
			name: "garbage collects",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact1.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
			},
			wantCollected: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
			},
			wantDeleted: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact1.tar.gz.lock"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz.lock"),
			},
			ctxTimeout: time.Second * 1,
		},
		{
			name: "garbage collection fails with context timeout",
			artifactPaths: []string{
				filepath.Join(artifactFolder, "artifact1.tar.gz"),
				filepath.Join(artifactFolder, "artifact2.tar.gz"),
				filepath.Join(artifactFolder, "artifact3.tar.gz"),
				filepath.Join(artifactFolder, "artifact4.tar.gz"),
			},
			wantErr:    "context deadline exceeded",
			ctxTimeout: time.Nanosecond * 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			dir := t.TempDir()

			opts := &config.Options{
				StoragePath:              dir,
				StorageAddress:           ":9090",
				ArtifactRetentionTTL:     time.Second * 2,
				ArtifactRetentionRecords: 2,
			}
			s, err := New(opts)
			g.Expect(err).ToNot(HaveOccurred(), "failed to create new storage")

			artifact := meta.Artifact{
				Path: tt.artifactPaths[len(tt.artifactPaths)-1],
			}
			g.Expect(os.MkdirAll(filepath.Join(dir, artifactFolder), 0o750)).ToNot(HaveOccurred())
			for i, artifactPath := range tt.artifactPaths {
				f, err := os.Create(filepath.Join(dir, artifactPath))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(f.Close()).ToNot(HaveOccurred())
				if i != len(tt.artifactPaths)-1 {
					time.Sleep(time.Second * 1)
				}
			}

			collectedPaths, err := s.GarbageCollect(context.TODO(), artifact, tt.ctxTimeout)
			if tt.wantErr == "" {
				g.Expect(err).ToNot(HaveOccurred(), "failed to collect garbage files")
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
			if len(tt.wantCollected) > 0 {
				g.Expect(len(tt.wantCollected)).To(Equal(len(collectedPaths)))
				for _, wantCollectedPath := range tt.wantCollected {
					present := false
					for _, collectedPath := range collectedPaths {
						if strings.Contains(collectedPath, wantCollectedPath) {
							g.Expect(collectedPath).ToNot(BeAnExistingFile())
							present = true
							break
						}
					}
					if present == false {
						g.Fail(fmt.Sprintf("expected file to be garbage collected, still exists: %s", wantCollectedPath))
					}
				}
			}
			for _, delFile := range tt.wantDeleted {
				g.Expect(filepath.Join(dir, delFile)).ToNot(BeAnExistingFile())
			}
		})
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
		g.Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
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

// walks a tar.gz and looks for paths with the basename. It does not match
// symlinks properly at this time because that's painful.
func walkTar(tarFile string, match string, dir bool) (int64, int64, bool, error) {
	f, err := os.Open(tarFile)
	if err != nil {
		return 0, 0, false, fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return 0, 0, false, fmt.Errorf("could not unzip file: %w", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return 0, 0, false, fmt.Errorf("corrupt tarball reading header: %w", err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if header.Name == match && dir {
				return 0, header.Mode, true, nil
			}
		case tar.TypeReg:
			if header.Name == match {
				return header.Size, header.Mode, true, nil
			}
		default:
			// skip
		}
	}

	return 0, 0, false, nil
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
