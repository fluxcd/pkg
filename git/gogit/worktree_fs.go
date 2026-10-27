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

package gogit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
)

// boundWorktreeFS is a BoundOS whose Remove and RemoveAll delete a symlink
// instead of its target (https://github.com/go-git/go-billy/issues/135).
// Remove once go-billy v6 is adopted.
type boundWorktreeFS struct {
	*osfs.BoundOS
}

// newBoundWorktreeFS returns a boundWorktreeFS rooted at path.
func newBoundWorktreeFS(path string) (*boundWorktreeFS, error) {
	bfs, ok := osfs.New(path, osfs.WithBoundOS()).(*osfs.BoundOS)
	if !ok {
		return nil, fmt.Errorf("unexpected filesystem type for %s", path)
	}
	return &boundWorktreeFS{BoundOS: bfs}, nil
}

// Remove removes the named file or empty directory without following a
// trailing symlink.
func (fs *boundWorktreeFS) Remove(name string) error {
	rel, ok, err := fs.entry(name)
	if err != nil {
		return err
	}
	if !ok {
		return fs.BoundOS.Remove(name)
	}
	root, err := os.OpenRoot(fs.Root())
	if err != nil {
		return err
	}
	return closeRoot(root, root.Remove(rel))
}

// RemoveAll removes name and any children it contains without following a
// trailing symlink.
func (fs *boundWorktreeFS) RemoveAll(name string) error {
	rel, ok, err := fs.entry(name)
	if err != nil {
		return err
	}
	if !ok {
		return fs.BoundOS.RemoveAll(name)
	}
	root, err := os.OpenRoot(fs.Root())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	return closeRoot(root, root.RemoveAll(rel))
}

// Chroot returns a boundWorktreeFS rooted at path within the current root.
func (fs *boundWorktreeFS) Chroot(path string) (billy.Filesystem, error) {
	c, err := fs.BoundOS.Chroot(path)
	if err != nil {
		return nil, err
	}
	bfs, ok := c.(*osfs.BoundOS)
	if !ok {
		return nil, fmt.Errorf("unexpected filesystem type for %s", path)
	}
	return &boundWorktreeFS{BoundOS: bfs}, nil
}

// entry returns the root-relative path of name, resolving all but the last
// component the same way BoundOS does. It returns false for names that
// refer to the root or end in "..".
func (fs *boundWorktreeFS) entry(name string) (string, bool, error) {
	base := fs.Root()
	name = fs.expandDot(name)

	// "l/" and "l/." refer to "l" itself.
	seps := string(filepath.Separator)
	if filepath.Separator != '/' {
		seps += "/"
	}
	for {
		trimmed := strings.TrimRight(name, seps)
		if trimmed == "" {
			break
		}
		if trimmed != "." && filepath.Base(trimmed) == "." {
			trimmed = trimmed[:len(trimmed)-1]
		}
		if trimmed == name {
			break
		}
		name = trimmed
	}
	if clean := filepath.Clean(name); clean == "." || clean == filepath.Clean(base) {
		return "", false, nil
	}

	for {
		dir, leaf := filepath.Split(name)
		if leaf == "" || leaf == "." || leaf == ".." {
			return "", false, nil
		}

		parent, err := securejoin.SecureJoin(base, dir)
		if err != nil {
			return "", false, err
		}
		path := filepath.Join(parent, leaf)

		// Same as the deduplicatePath handling in BoundOS.abs.
		vol := filepath.VolumeName(base)
		dup := filepath.Join(base, base[len(vol):])
		if strings.HasPrefix(path, dup+string(filepath.Separator)) {
			name = path[len(dup):]
			continue
		}

		rel, err := filepath.Rel(base, path)
		if err != nil {
			return "", false, err
		}
		return rel, true, nil
	}
}

// expandDot is a copy of BoundOS.expandDot.
func (fs *boundWorktreeFS) expandDot(name string) string {
	if name == "." {
		return fs.Root()
	}
	prefixes, seps := []string{"./"}, "/"
	if filepath.Separator == '\\' {
		prefixes, seps = []string{"./", ".\\"}, `/\`
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			name = strings.TrimLeft(strings.TrimPrefix(name, prefix), seps)
			if name == "" {
				return fs.Root()
			}
			return name
		}
	}
	return name
}

// closeRoot closes root and returns err, or the close error if err is nil.
func closeRoot(root *os.Root, err error) error {
	if cerr := root.Close(); err == nil {
		return cerr
	}
	return err
}
