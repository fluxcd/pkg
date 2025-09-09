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
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/artifact/config"
	. "github.com/fluxcd/pkg/artifact/storage"
)

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
