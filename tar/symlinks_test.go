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
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSymlinks_directoryWithSymlinkToFile(t *testing.T) {
	// external/real.txt  <-- symlink target outside source
	// src/link.txt -> ../external/real.txt
	// src/regular.txt
	root := t.TempDir()
	external := filepath.Join(root, "external")
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "real.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "regular.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(external, "real.txt"), filepath.Join(src, "link.txt")); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ResolveSymlinks(src, dst); err != nil {
		t.Fatalf("ResolveSymlinks: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "link.txt"))
	if err != nil {
		t.Fatalf("read link.txt: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("link.txt content: got %q, want %q", got, "hello")
	}

	got, err = os.ReadFile(filepath.Join(dst, "regular.txt"))
	if err != nil {
		t.Fatalf("read regular.txt: %v", err)
	}
	if string(got) != "world" {
		t.Errorf("regular.txt content: got %q, want %q", got, "world")
	}

	// The staged tree must contain no symlinks.
	fi, err := os.Lstat(filepath.Join(dst, "link.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Errorf("staged link.txt is still a symlink")
	}
}

func TestResolveSymlinks_rejectsFileInput(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "plain.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ResolveSymlinks(file, t.TempDir())
	if err == nil {
		t.Fatal("expected error for file input")
	}
}

func TestResolveSymlinks_symlinkToDir(t *testing.T) {
	// external/{a.txt, b.txt}
	// src/nested -> ../external
	root := t.TempDir()
	external := filepath.Join(root, "external")
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "a.txt"), []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "b.txt"), []byte("B"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(src, "nested")); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ResolveSymlinks(src, dst); err != nil {
		t.Fatalf("ResolveSymlinks: %v", err)
	}

	for name, want := range map[string]string{"a.txt": "A", "b.txt": "B"} {
		got, err := os.ReadFile(filepath.Join(dst, "nested", name))
		if err != nil {
			t.Fatalf("read nested/%s: %v", name, err)
		}
		if string(got) != want {
			t.Errorf("nested/%s: got %q, want %q", name, got, want)
		}
	}
}

func TestResolveSymlinks_cycle(t *testing.T) {
	// src/
	//   a/
	//     loop -> ../b   (symlink)
	//   b/
	//     loop -> ../a   (symlink)
	//     file.txt
	root := t.TempDir()
	src := filepath.Join(root, "src")
	a := filepath.Join(src, "a")
	b := filepath.Join(src, "b")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "file.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(b, filepath.Join(a, "loop")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, filepath.Join(b, "loop")); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ResolveSymlinks(src, dst); err != nil {
		t.Fatalf("ResolveSymlinks: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "b", "file.txt"))
	if err != nil {
		t.Fatalf("read b/file.txt: %v", err)
	}
	if string(got) != "hi" {
		t.Errorf("b/file.txt: got %q, want %q", got, "hi")
	}
}

func TestResolveSymlinks_thenTar(t *testing.T) {
	// End-to-end: resolve a tree with external symlinks, then tar.
	root := t.TempDir()
	external := filepath.Join(root, "external")
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "real.yaml"), []byte("kind: Thing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(external, "real.yaml"), filepath.Join(src, "manifest.yaml")); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ResolveSymlinks(src, dst); err != nil {
		t.Fatalf("ResolveSymlinks: %v", err)
	}

	var buf bytes.Buffer
	if _, err := Tar(dst, &buf); err != nil {
		t.Fatalf("Tar: %v", err)
	}

	got := readTarEntries(t, &buf)
	if got["manifest.yaml"] != "kind: Thing" {
		t.Errorf("manifest.yaml: got %q, want %q", got["manifest.yaml"], "kind: Thing")
	}
}

func TestResolveSymlinks_nonexistent(t *testing.T) {
	err := ResolveSymlinks("/definitely/not/a/real/path/xyzzy", t.TempDir())
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestResolveSymlinks_rejectsMissingDst(t *testing.T) {
	src := t.TempDir()
	err := ResolveSymlinks(src, filepath.Join(t.TempDir(), "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for missing dstDir")
	}
}

func TestResolveSymlinks_rejectsDstFile(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(dst, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := ResolveSymlinks(src, dst)
	if err == nil {
		t.Fatal("expected error for dstDir that is a file")
	}
}

func TestResolveSymlinksRoot_resolvesWithinRoot(t *testing.T) {
	// root/
	//   src/link.txt -> ../external/real.txt
	//   external/real.txt
	root := t.TempDir()
	src := filepath.Join(root, "src")
	external := filepath.Join(root, "external")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "real.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(external, "real.txt"), filepath.Join(src, "link.txt")); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ResolveSymlinksRoot(root, src, dst); err != nil {
		t.Fatalf("ResolveSymlinksRoot: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "link.txt"))
	if err != nil {
		t.Fatalf("read link.txt: %v", err)
	}
	if string(got) != "ok" {
		t.Errorf("content: got %q, want %q", got, "ok")
	}
}

func TestResolveSymlinksRoot_rejectsEscape(t *testing.T) {
	// root/src/escape -> /tmp/<outside>/target.txt
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}

	outside := t.TempDir() // sibling temp dir, outside root
	target := filepath.Join(outside, "target.txt")
	if err := os.WriteFile(target, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(src, "escape")); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	err := ResolveSymlinksRoot(root, src, dst)
	if err == nil {
		t.Fatal("expected error for symlink escaping root")
	}

	// The escaped content must not have been materialized.
	if _, statErr := os.Stat(filepath.Join(dst, "escape")); statErr == nil {
		t.Error("escape target was materialized in dstDir")
	}
}

func TestResolveSymlinksRoot_rejectsSrcOutsideRoot(t *testing.T) {
	root := t.TempDir()
	otherRoot := t.TempDir()

	err := ResolveSymlinksRoot(root, otherRoot, t.TempDir())
	if err == nil {
		t.Fatal("expected error when srcDir is outside rootDir")
	}
}

func TestResolveSymlinksRoot_cycle(t *testing.T) {
	// Cycle within the root should terminate.
	root := t.TempDir()
	src := filepath.Join(root, "src")
	a := filepath.Join(src, "a")
	b := filepath.Join(src, "b")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(b, "file.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(b, filepath.Join(a, "loop")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, filepath.Join(b, "loop")); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ResolveSymlinksRoot(root, src, dst); err != nil {
		t.Fatalf("ResolveSymlinksRoot: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dst, "b", "file.txt"))
	if err != nil {
		t.Fatalf("read b/file.txt: %v", err)
	}
	if string(got) != "hi" {
		t.Errorf("b/file.txt: got %q, want %q", got, "hi")
	}
}

func TestResolveSymlinksRoot_thenTar(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	external := filepath.Join(root, "external")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(external, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(external, "real.yaml"), []byte("kind: Thing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(external, "real.yaml"), filepath.Join(src, "manifest.yaml")); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ResolveSymlinksRoot(root, src, dst); err != nil {
		t.Fatalf("ResolveSymlinksRoot: %v", err)
	}

	var buf bytes.Buffer
	if _, err := Tar(dst, &buf); err != nil {
		t.Fatalf("Tar: %v", err)
	}

	got := readTarEntries(t, &buf)
	if got["manifest.yaml"] != "kind: Thing" {
		t.Errorf("manifest.yaml: got %q, want %q", got["manifest.yaml"], "kind: Thing")
	}
}
