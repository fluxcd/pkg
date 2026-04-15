/*
Copyright 2022 The Flux authors

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

package tar

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

type untarTestCase struct {
	name            string
	targetDir       string
	secureTargetDir string
	fileName        string
	content         []byte
	wantErr         string
	maxUntarSize    int
	fileMode        int64
}

func TestUntar(t *testing.T) {
	targetDirOutput := filepath.Join(t.TempDir(), "output")
	symlink := filepath.Join(t.TempDir(), "symlink")

	subdir := filepath.Join(targetDirOutput, "subdir")
	err := os.MkdirAll(subdir, 0o750)
	if err != nil {
		t.Fatalf("cannot create subdir: %v", err)
	}

	err = os.Symlink(subdir, symlink)
	if err != nil {
		t.Fatalf("cannot create symlink: %v", err)
	}

	cases := []untarTestCase{
		{
			name:            "file at root",
			fileName:        "file1",
			content:         geRandomContent(256),
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "file at subdir root",
			fileName:        "abc/fileX",
			content:         geRandomContent(256),
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "directory traversal parent",
			fileName:        "../abc/file",
			content:         geRandomContent(256),
			wantErr:         `tar contained invalid name error "../abc/file"`,
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "breach max size",
			fileName:        "big-file",
			content:         geRandomContent(256),
			maxUntarSize:    255,
			wantErr:         `tar "big-file" is bigger than max archive size of 255 bytes`,
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "breach default max untar size",
			fileName:        "another-big-file",
			content:         geRandomContent(1024),
			maxUntarSize:    512,
			wantErr:         `tar "another-big-file" is bigger than max archive size of 512 bytes`,
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "disable max size checks",
			fileName:        "another-big-file",
			content:         geRandomContent(1024),
			maxUntarSize:    -1,
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "existing subdir",
			fileName:        "subdir/file1",
			content:         geRandomContent(256),
			targetDir:       targetDirOutput,
			secureTargetDir: targetDirOutput,
		},
		{
			name:            "relative target dir",
			fileName:        "file1",
			content:         geRandomContent(256),
			targetDir:       "anydir",
			secureTargetDir: "./anydir",
		},
		{
			name:            "relative paths can't ascend",
			fileName:        "file1",
			content:         geRandomContent(256),
			targetDir:       "../../../../../../../../tmp/test",
			secureTargetDir: "./tmp/test",
		},
		{
			name:      "symlink",
			fileName:  "any-file1",
			content:   geRandomContent(256),
			targetDir: symlink,
			wantErr:   fmt.Sprintf(`dir '%s' must be a directory`, symlink),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := createTestTar(t, tt)
			defer os.RemoveAll(tt.targetDir)

			opts := make([]Option, 0)
			if tt.maxUntarSize != 0 {
				opts = append(opts, WithMaxUntarSize(tt.maxUntarSize))
			}

			err = Untar(f, tt.targetDir, opts...)
			var got string
			if err != nil {
				got = err.Error()
			}
			if tt.wantErr != got {
				t.Errorf("wanted error: '%s' got: '%v'", tt.wantErr, err)
			}

			if tt.wantErr == "" && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// only assess file if no errors were expected
			if tt.wantErr == "" {
				abs := filepath.Join(tt.secureTargetDir, tt.fileName)
				fi, err := os.Stat(abs)
				if err != nil {
					t.Errorf("stat %q: %v", abs, err)
					return
				}

				if fi.Size() != int64(len(tt.content)) {
					t.Errorf("file size wanted: %d got: %d", len(tt.content), fi.Size())
				}
			}

			if tt.targetDir != tt.secureTargetDir {
				os.RemoveAll(tt.secureTargetDir)
			}
		})
	}
}

func TestUntarDirectoryPermissions(t *testing.T) {
	testDirName := "test-dir"

	f := createTestTar(t, untarTestCase{
		fileName: testDirName + "/", // from tar.Header: a trailing slash makes the entry a TypeDir
		fileMode: 0o555,
		content:  nil,
	})

	targetDir := t.TempDir()

	if err := Untar(f, targetDir); err != nil {
		t.Fatalf("untar: %v", err)
	}

	fullPath := filepath.Join(targetDir, testDirName)
	fi, err := os.Lstat(fullPath)
	if err != nil {
		t.Errorf("stat %q: %v", fullPath, err)
	}

	if !fi.Mode().IsDir() {
		t.Fatalf("%q: not a directory", fullPath)
	}

	ownerPerm := fi.Mode().Perm() & 0o700
	if ownerPerm != 0o700 {
		t.Errorf("the owner must always be able to traverse, read, and write extracted directories")
	}
}

func Fuzz_Untar(f *testing.F) {
	tf := createTestTar(f, untarTestCase{
		name:     "file at root",
		fileName: "file1",
		content:  geRandomContent(256),
	})

	var content []byte
	if _, err := tf.Read(content); err != nil {
		f.Fatalf("cannot read test tar: %v", err)
	}

	f.Add(content)

	f.Fuzz(func(t *testing.T, data []byte) {
		_ = Untar(bytes.NewReader(data), t.TempDir())
	})
}

func createTestTar(t testing.TB, tt untarTestCase) *os.File {
	t.Helper()

	name := filepath.Join(t.TempDir(), "test.tar.gz")
	f, err := os.Create(name)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}

	gzw := gzip.NewWriter(f)
	writer := tar.NewWriter(gzw)

	fileMode := tt.fileMode
	if fileMode == 0 {
		fileMode = 0o777
	}

	writer.WriteHeader(&tar.Header{
		Name: tt.fileName,
		Size: int64(len(tt.content)),
		Mode: fileMode,
	})

	writer.Write(tt.content)

	if err = writer.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err = gzw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	if err = f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	f, err = os.Open(name)
	if err != nil {
		t.Fatalf("reopen file: %v", err)
	}
	return f
}

func geRandomContent(n int) []byte {
	content := make([]byte, n)
	for i := range content {
		content[i] = byte(i % 251)
	}
	return content
}

func TestSkipSymlinks(t *testing.T) {
	tmpDir := t.TempDir()

	symlinkTarget := filepath.Join(tmpDir, "symlink.target")
	err := os.WriteFile(symlinkTarget, geRandomContent(256), os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	symlink := filepath.Join(tmpDir, "symlink")
	err = os.Symlink(symlinkTarget, symlink)
	if err != nil {
		t.Fatal(err)
	}

	tgzFileName := filepath.Join(t.TempDir(), "test.tgz")
	var buf bytes.Buffer
	err = tgzWithSymlinks(tmpDir, &buf)
	if err != nil {
		t.Fatal(err)
	}

	tgzFile, err := os.OpenFile(tgzFileName, os.O_CREATE|os.O_RDWR, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(tgzFile, &buf); err != nil {
		t.Fatal(err)
	}
	if err = tgzFile.Close(); err != nil {
		t.Fatal(err)
	}

	targetDirOutput := filepath.Join(t.TempDir(), "output")
	f1, err := os.Open(tgzFileName)
	if err != nil {
		t.Fatal(err)
	}

	err = Untar(f1, targetDirOutput, WithMaxUntarSize(-1))
	if err == nil {
		t.Errorf("wanted error: unsupported symlink")
	}

	f2, err := os.Open(tgzFileName)
	if err != nil {
		t.Fatal(err)
	}

	err = Untar(f2, targetDirOutput, WithMaxUntarSize(-1), WithSkipSymlinks())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if _, err := os.Open(path.Join(targetDirOutput, "symlink.target")); err != nil {
		t.Errorf("regular file not found: %v", err)
	}
}

func tgzWithSymlinks(src string, buf io.Writer) error {
	absDir, err := filepath.Abs(src)
	if err != nil {
		return err
	}

	zr := gzip.NewWriter(buf)
	tw := tar.NewWriter(zr)
	if err := filepath.Walk(absDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, file)
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if fi.Mode().IsRegular() {
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
			return f.Close()
		}

		return nil
	}); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if err := zr.Close(); err != nil {
		return err
	}
	return nil
}

func TestUntar_withFilter(t *testing.T) {
	// Build a gzipped tar with two files.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, data := range map[string]string{"keep.txt": "keep", "skip.log": "skip"} {
		tw.WriteHeader(&tar.Header{Name: name, Size: int64(len(data)), Mode: 0o644})
		tw.Write([]byte(data))
	}
	tw.Close()
	gw.Close()

	dst := t.TempDir()
	filter := func(p string, _ os.FileInfo) bool {
		return filepath.Ext(p) == ".log"
	}
	if err := Untar(&buf, dst, WithMaxUntarSize(-1), WithFilter(filter)); err != nil {
		t.Fatalf("Untar: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "skip.log")); err == nil {
		t.Error("filtered file skip.log should not have been extracted")
	}
	got, err := os.ReadFile(filepath.Join(dst, "keep.txt"))
	if err != nil {
		t.Fatalf("read keep.txt: %v", err)
	}
	if string(got) != "keep" {
		t.Errorf("keep.txt: got %q, want %q", string(got), "keep")
	}
}

func TestUntar_withFilterDirectory(t *testing.T) {
	// Build a gzipped tar with entries under two directories.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	tw.WriteHeader(&tar.Header{Name: "skip/", Mode: 0o755, Typeflag: tar.TypeDir})
	s := "secret"
	tw.WriteHeader(&tar.Header{Name: "skip/secret.txt", Size: int64(len(s)), Mode: 0o644})
	tw.Write([]byte(s))
	tw.WriteHeader(&tar.Header{Name: "keep/", Mode: 0o755, Typeflag: tar.TypeDir})
	k := "public"
	tw.WriteHeader(&tar.Header{Name: "keep/data.txt", Size: int64(len(k)), Mode: 0o644})
	tw.Write([]byte(k))
	tw.Close()
	gw.Close()

	dst := t.TempDir()
	filter := func(p string, _ os.FileInfo) bool {
		return strings.HasPrefix(p, "skip/")
	}
	if err := Untar(&buf, dst, WithMaxUntarSize(-1), WithFilter(filter)); err != nil {
		t.Fatalf("Untar: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "skip", "secret.txt")); err == nil {
		t.Error("skip/secret.txt should not have been extracted")
	}
	got, err := os.ReadFile(filepath.Join(dst, "keep", "data.txt"))
	if err != nil {
		t.Fatalf("read keep/data.txt: %v", err)
	}
	if string(got) != "public" {
		t.Errorf("keep/data.txt: got %q, want %q", string(got), "public")
	}
}
