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

package filesys

import (
	"os"
	"path/filepath"
	"testing"

	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func testTemp(t *testing.T) string {
	t.Helper()
	tmp, err := testTempDir(t)
	if err != nil {
		t.Fatal(err)
	}
	return tmp
}

func Test_fsMemory_ReadFile_fromDisk(t *testing.T) {
	tmp := testTemp(t)
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("disk"), 0o644); err != nil {
		t.Fatal(err)
	}

	diskFS, err := MakeFsOnDiskSecure(tmp)
	if err != nil {
		t.Fatal(err)
	}
	fs := MakeFsInMemory(diskFS)

	data, err := fs.ReadFile(filepath.Join(tmp, "a.txt"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "disk" {
		t.Errorf("got %q, want %q", data, "disk")
	}
}

func Test_fsMemory_WriteFile_toMemory(t *testing.T) {
	tmp := testTemp(t)

	diskFS, err := MakeFsOnDiskSecure(tmp)
	if err != nil {
		t.Fatal(err)
	}
	fs := MakeFsInMemory(diskFS)

	path := filepath.Join(tmp, "new.txt")
	if err := fs.WriteFile(path, []byte("memory")); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Readable from memory fs.
	data, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "memory" {
		t.Errorf("got %q, want %q", data, "memory")
	}

	// Not written to disk.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file should not exist on disk, got err: %v", err)
	}
}

func Test_fsMemory_memoryOverridesDisk(t *testing.T) {
	tmp := testTemp(t)
	path := filepath.Join(tmp, "f.txt")
	if err := os.WriteFile(path, []byte("disk"), 0o644); err != nil {
		t.Fatal(err)
	}

	diskFS, err := MakeFsOnDiskSecure(tmp)
	if err != nil {
		t.Fatal(err)
	}
	fs := MakeFsInMemory(diskFS)

	if err := fs.WriteFile(path, []byte("memory")); err != nil {
		t.Fatal(err)
	}

	data, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "memory" {
		t.Errorf("got %q, want %q", data, "memory")
	}
}

func Test_fsMemory_Exists(t *testing.T) {
	tmp := testTemp(t)
	if err := os.WriteFile(filepath.Join(tmp, "disk.txt"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}

	diskFS, err := MakeFsOnDiskSecure(tmp)
	if err != nil {
		t.Fatal(err)
	}
	fs := MakeFsInMemory(diskFS)

	if err := fs.WriteFile(filepath.Join(tmp, "mem.txt"), []byte("m")); err != nil {
		t.Fatal(err)
	}

	if !fs.Exists(filepath.Join(tmp, "disk.txt")) {
		t.Error("disk file should exist")
	}
	if !fs.Exists(filepath.Join(tmp, "mem.txt")) {
		t.Error("memory file should exist")
	}
	if fs.Exists(filepath.Join(tmp, "nope.txt")) {
		t.Error("non-existent file should not exist")
	}
}

func Test_fsMemory_IsDir(t *testing.T) {
	tmp := testTemp(t)
	if err := os.MkdirAll(filepath.Join(tmp, "diskdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	diskFS, err := MakeFsOnDiskSecure(tmp)
	if err != nil {
		t.Fatal(err)
	}
	fs := MakeFsInMemory(diskFS)

	if err := fs.MkdirAll(filepath.Join(tmp, "memdir")); err != nil {
		t.Fatal(err)
	}

	if !fs.IsDir(filepath.Join(tmp, "diskdir")) {
		t.Error("disk dir should be a dir")
	}
	if !fs.IsDir(filepath.Join(tmp, "memdir")) {
		t.Error("memory dir should be a dir")
	}
}

func Test_fsMemory_ReadDir_merged(t *testing.T) {
	tmp := testTemp(t)
	if err := os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	diskFS, err := MakeFsOnDiskSecure(tmp)
	if err != nil {
		t.Fatal(err)
	}
	fs := MakeFsInMemory(diskFS)

	if err := fs.WriteFile(filepath.Join(tmp, "b.txt"), []byte("b")); err != nil {
		t.Fatal(err)
	}

	entries, err := fs.ReadDir(tmp)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	has := func(name string) bool {
		for _, e := range entries {
			if e == name {
				return true
			}
		}
		return false
	}
	if !has("a.txt") {
		t.Error("should contain disk file a.txt")
	}
	if !has("b.txt") {
		t.Error("should contain memory file b.txt")
	}
}

func Test_fsMemory_diskSecurityConstraint(t *testing.T) {
	tmp := testTemp(t)

	diskFS, err := MakeFsOnDiskSecure(tmp)
	if err != nil {
		t.Fatal(err)
	}
	fs := MakeFsInMemory(diskFS)

	// Reading outside the secure root should fail.
	_, err = fs.ReadFile("/etc/passwd")
	if err == nil {
		t.Error("expected error reading outside secure root")
	}
}

func Test_fsMemory_Walk(t *testing.T) {
	tmp := testTemp(t)
	if err := os.WriteFile(filepath.Join(tmp, "disk.txt"), []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}

	fs := MakeFsInMemory(filesys.MakeFsOnDisk())
	if err := fs.WriteFile(filepath.Join(tmp, "mem.txt"), []byte("m")); err != nil {
		t.Fatal(err)
	}

	visited := map[string]bool{}
	err := fs.Walk(tmp, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(tmp, p)
		visited[rel] = true
		return nil
	})
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	if !visited["disk.txt"] {
		t.Error("should visit disk.txt")
	}
	if !visited["mem.txt"] {
		t.Error("should visit mem.txt")
	}
}
