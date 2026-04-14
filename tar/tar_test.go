/*
Copyright 2026 The Flux authors

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
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestTar(t *testing.T) {
	srcDir := t.TempDir()

	// Create test files.
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "nested.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	n, err := Tar(srcDir, &buf)
	if err != nil {
		t.Fatalf("Tar() error: %v", err)
	}
	if n <= 0 {
		t.Fatal("Tar() returned zero bytes")
	}
	if n != int64(buf.Len()) {
		t.Fatalf("Tar() returned %d bytes, but buffer has %d", n, buf.Len())
	}

	got := readTarEntries(t, &buf)
	want := map[string]string{
		".":                 "",
		"file.txt":          "hello",
		"subdir":            "",
		"subdir/nested.txt": "world",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %v", len(got), len(want), got)
	}
	for name, content := range want {
		if got[name] != content {
			t.Errorf("entry %q: got %q, want %q", name, got[name], content)
		}
	}
}

func TestTar_roundTrip(t *testing.T) {
	srcDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcDir, "a", "b"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a", "b", "c.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := Tar(srcDir, &buf); err != nil {
		t.Fatalf("Tar() error: %v", err)
	}

	dstDir := t.TempDir()
	if err := Untar(&buf, dstDir); err != nil {
		t.Fatalf("Untar() error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dstDir, "a", "b", "c.txt"))
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if string(got) != "data" {
		t.Fatalf("got %q, want %q", got, "data")
	}
}

func TestTar_sanitizesHeaders(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := Tar(srcDir, &buf); err != nil {
		t.Fatal(err)
	}

	gr, err := gzip.NewReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if hdr.Uid != 0 || hdr.Gid != 0 {
			t.Errorf("entry %q: uid=%d gid=%d, want 0", hdr.Name, hdr.Uid, hdr.Gid)
		}
		if hdr.Uname != "" || hdr.Gname != "" {
			t.Errorf("entry %q: uname=%q gname=%q, want empty", hdr.Name, hdr.Uname, hdr.Gname)
		}
		if !hdr.ModTime.IsZero() && hdr.ModTime.Unix() != 0 {
			t.Errorf("entry %q: ModTime=%v, want zero or Unix epoch", hdr.Name, hdr.ModTime)
		}
	}
}

func TestTar_withFilter(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "skip.log"), []byte("skip"), 0o644); err != nil {
		t.Fatal(err)
	}

	filter := func(p string, fi os.FileInfo) bool {
		return filepath.Ext(p) == ".log"
	}

	var buf bytes.Buffer
	if _, err := Tar(srcDir, &buf, WithFilter(filter)); err != nil {
		t.Fatal(err)
	}

	got := readTarEntries(t, &buf)
	if _, ok := got["skip.log"]; ok {
		t.Error("filtered file skip.log should not be in archive")
	}
	if got["keep.txt"] != "keep" {
		t.Errorf("keep.txt: got %q, want %q", got["keep.txt"], "keep")
	}
}

func TestTar_skipsSymlinks(t *testing.T) {
	srcDir := t.TempDir()
	target := filepath.Join(srcDir, "target.txt")
	if err := os.WriteFile(target, []byte("t"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(srcDir, "link.txt")); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := Tar(srcDir, &buf); err != nil {
		t.Fatal(err)
	}

	got := readTarEntries(t, &buf)
	if _, ok := got["link.txt"]; ok {
		t.Error("symlink should not be in archive")
	}
	if got["target.txt"] != "t" {
		t.Errorf("target.txt: got %q, want %q", got["target.txt"], "t")
	}
}

func TestTar_invalidDir(t *testing.T) {
	_, err := Tar("/nonexistent/path", io.Discard)
	if err == nil {
		t.Fatal("expected error for nonexistent dir")
	}
}

func TestTar_skipGzip(t *testing.T) {
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("plain"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := Tar(srcDir, &buf, WithSkipGzip()); err != nil {
		t.Fatalf("Tar() error: %v", err)
	}

	// Should be a valid plain tar, not gzip.
	tr := tar.NewReader(&buf)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		if hdr.Name == "file.txt" {
			found = true
			content, _ := io.ReadAll(tr)
			if string(content) != "plain" {
				t.Errorf("got %q, want %q", content, "plain")
			}
		}
	}
	if !found {
		t.Error("file.txt not found in plain tar")
	}
}

func TestTar_notADirectory(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Tar(f, io.Discard)
	if err == nil {
		t.Fatal("expected error for file path")
	}
}

// readTarEntries decompresses a tar.gz and returns a map of entry name to content.
func readTarEntries(t *testing.T, r io.Reader) map[string]string {
	t.Helper()
	gr, err := gzip.NewReader(r)
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	defer gr.Close()

	entries := make(map[string]string)
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("tar.Next: %v", err)
		}
		var content []byte
		if hdr.Typeflag == tar.TypeReg {
			content, err = io.ReadAll(tr)
			if err != nil {
				t.Fatalf("ReadAll %q: %v", hdr.Name, err)
			}
		}
		entries[hdr.Name] = string(content)
	}
	return entries
}
