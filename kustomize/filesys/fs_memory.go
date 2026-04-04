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

	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// MakeFsInMemory returns a filesystem that reads from disk and writes to
// memory. Read operations check the memory layer first, then fall back to the
// disk layer. Write operations only go to the memory layer, so the on-disk
// files are never modified.
//
// The disk layer can be any filesys.FileSystem (e.g., an fsSecure instance to
// constrain reads to a root directory).
func MakeFsInMemory(disk filesys.FileSystem) filesys.FileSystem {
	return fsMemory{disk: disk, memory: filesys.MakeFsInMemory()}
}

// fsMemory layers an in-memory filesystem on top of a read-only disk
// filesystem. Writes go to memory; reads check memory first, then disk.
type fsMemory struct {
	disk   filesys.FileSystem
	memory filesys.FileSystem
}

// Write operations: memory only.

func (fs fsMemory) Create(path string) (filesys.File, error) {
	return fs.memory.Create(path)
}
func (fs fsMemory) Mkdir(path string) error                  { return fs.memory.Mkdir(path) }
func (fs fsMemory) MkdirAll(path string) error               { return fs.memory.MkdirAll(path) }
func (fs fsMemory) RemoveAll(path string) error              { return fs.memory.RemoveAll(path) }
func (fs fsMemory) WriteFile(path string, data []byte) error { return fs.memory.WriteFile(path, data) }

// Read operations: memory first, then disk.

func (fs fsMemory) Exists(path string) bool { return fs.memory.Exists(path) || fs.disk.Exists(path) }
func (fs fsMemory) IsDir(path string) bool  { return fs.memory.IsDir(path) || fs.disk.IsDir(path) }

func (fs fsMemory) Open(path string) (filesys.File, error) {
	if fs.memory.Exists(path) {
		return fs.memory.Open(path)
	}
	return fs.disk.Open(path)
}

func (fs fsMemory) ReadFile(path string) ([]byte, error) {
	if fs.memory.Exists(path) {
		return fs.memory.ReadFile(path)
	}
	return fs.disk.ReadFile(path)
}

func (fs fsMemory) CleanedAbs(path string) (filesys.ConfirmedDir, string, error) {
	return fs.disk.CleanedAbs(path)
}

func (fs fsMemory) ReadDir(path string) ([]string, error) {
	return mergeResults(fs.memory.ReadDir(path))(fs.disk.ReadDir(path))
}

func (fs fsMemory) Glob(pattern string) ([]string, error) {
	return mergeResults(fs.memory.Glob(pattern))(fs.disk.Glob(pattern))
}

func (fs fsMemory) Walk(path string, walkFn filepath.WalkFunc) error {
	visited := make(map[string]struct{})
	if fs.memory.Exists(path) {
		if err := fs.memory.Walk(path, func(p string, info os.FileInfo, err error) error {
			visited[p] = struct{}{}
			return walkFn(p, info, err)
		}); err != nil {
			return err
		}
	}
	return fs.disk.Walk(path, func(p string, info os.FileInfo, err error) error {
		if _, ok := visited[p]; ok {
			return nil
		}
		return walkFn(p, info, err)
	})
}

// mergeResults deduplicates two ([]string, error) results, preferring the
// first set. Returns a closure so both calls can be inlined at the call site.
func mergeResults(primary []string, pErr error) func([]string, error) ([]string, error) {
	return func(secondary []string, sErr error) ([]string, error) {
		if pErr != nil && sErr != nil {
			return nil, sErr
		}
		seen := make(map[string]struct{}, len(primary))
		merged := make([]string, 0, len(primary)+len(secondary))
		for _, e := range primary {
			seen[e] = struct{}{}
			merged = append(merged, e)
		}
		for _, e := range secondary {
			if _, ok := seen[e]; !ok {
				merged = append(merged, e)
			}
		}
		return merged, nil
	}
}
