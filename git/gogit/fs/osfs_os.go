/*
Copyright 2017 Go-Git authors.

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

// Copyright 2022 The Flux authors. All rights reserved.
// Adapted from: github.com/go-git/go-billy/v5/osfs

package fs

import (
	"fmt"
	stdfs "io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-git/go-billy/v5"
)

const (
	defaultDirectoryMode = 0o755
	defaultCreateMode    = 0o666
)

// OS is a fs implementation based on the OS filesystem which has some
// changes in behaviour when compared to the upstream go-git/go-billy/v5/osfs:
//
// - Chroot is not supported and paths are not changed from the underlying OS fs.
// - Relative paths are forced to descend from the working dir.
// - Symlinks don't have its targets modified, and therefore can point to locations
// outside the working dir or to non-existent paths.
// - OpenFile honours the FileMode passed as argument.
// - ReadLink and Lstat does not follow symlinks as most other funcs do.
// However, it ensures that:
//
//	a) The filename is located within the current dir.
//	b) The dir in which filename is based, is located within the current dir.
type OS struct {
	workingDir string
}

// New returns a new OS filesystem using the workingDir as prefix for relative paths.
// It also ensures that operations are kept within that working dir.
func New(workingDir string) billy.Filesystem {
	return &OS{
		workingDir: workingDir,
	}
}

func (fs *OS) Create(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func (fs *OS) OpenFile(filename string, flag int, perm os.FileMode) (billy.File, error) {
	fn, err := fs.abs(filename)
	if err != nil {
		return nil, err
	}
	if flag&os.O_CREATE != 0 {
		if err := fs.createDir(fn); err != nil {
			return nil, err
		}
	}

	f, err := os.OpenFile(fn, flag, perm)
	if err != nil {
		return nil, err
	}
	return &file{File: f}, err
}

func (fs *OS) ReadDir(path string) ([]os.FileInfo, error) {
	dir, err := fs.abs(path)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	infos := make([]stdfs.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (fs *OS) Rename(from, to string) error {
	f, err := fs.abs(from)
	if err != nil {
		return err
	}
	t, err := fs.abs(to)
	if err != nil {
		return err
	}

	// MkdirAll for target name.
	if err := fs.createDir(t); err != nil {
		return err
	}

	return os.Rename(f, t)
}

func (fs *OS) MkdirAll(path string, perm os.FileMode) error {
	dir, err := fs.abs(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, perm)
}

func (fs *OS) Open(filename string) (billy.File, error) {
	return fs.OpenFile(filename, os.O_RDONLY, 0)
}

func (fs *OS) Stat(filename string) (os.FileInfo, error) {
	filename, err := fs.abs(filename)
	if err != nil {
		return nil, err
	}
	return os.Stat(filename)
}

func (fs *OS) Remove(filename string) error {
	fn, err := fs.abs(filename)
	if err != nil {
		return err
	}
	return os.Remove(fn)
}

// TempFile creates a temporary file. If dir is empty, the file
// will be created within the OS Temporary dir. If dir is provided
// it must descend from the current working dir.
func (fs *OS) TempFile(dir, prefix string) (billy.File, error) {
	if dir != "" {
		var err error
		dir, err = fs.abs(dir)
		if err != nil {
			return nil, err
		}
	}

	f, err := os.CreateTemp(dir, prefix)
	if err != nil {
		return nil, err
	}
	return &file{File: f}, nil
}

func (fs *OS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *OS) RemoveAll(path string) error {
	dir, err := fs.abs(path)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func (fs *OS) Symlink(target, link string) error {
	ln, err := fs.abs(link)
	if err != nil {
		return err
	}
	// MkdirAll for containing dir.
	if err := fs.createDir(ln); err != nil {
		return err
	}
	return os.Symlink(target, ln)
}

func (fs *OS) Lstat(filename string) (os.FileInfo, error) {
	filename = filepath.Clean(filename)
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(fs.workingDir, filename)
	}
	if ok, err := fs.insideWorkingDirEval(filename); !ok {
		return nil, err
	}
	return os.Lstat(filename)
}

func (fs *OS) Readlink(link string) (string, error) {
	if !filepath.IsAbs(link) {
		link = filepath.Clean(filepath.Join(fs.workingDir, link))
	}
	if ok, err := fs.insideWorkingDirEval(link); !ok {
		return "", err
	}
	return os.Readlink(link)
}

func (fs *OS) Chroot(path string) (billy.Filesystem, error) {
	return nil, billy.ErrNotSupported
}

// Root returns the current working dir of the billy.Filesystem.
// This is required in order for this implementation to be a drop-in
// replacement for other upstream implementations (e.g. memory and osfs).
func (fs *OS) Root() string {
	return fs.workingDir
}

// file is a wrapper for an os.File which adds support for file locking.
type file struct {
	*os.File
	m sync.Mutex
}

func (fs *OS) createDir(fullpath string) error {
	dir := filepath.Dir(fullpath)
	if dir != "." {
		if err := os.MkdirAll(dir, defaultDirectoryMode); err != nil {
			return err
		}
	}

	return nil
}

// abs transforms filename to an absolute path, taking into account the working dir.
// Relative paths won't be allowed to ascend the working dir, so `../file` will become
// `/working-dir/file`.
//
// Note that if filename is a symlink, the returned address will be the target of the
// symlink.
func (fs *OS) abs(filename string) (string, error) {
	if filename == fs.workingDir {
		filename = "/"
	} else if strings.HasPrefix(filename, fs.workingDir+string(filepath.Separator)) {
		filename = strings.TrimPrefix(filename, fs.workingDir+string(filepath.Separator))
	}
	return SecureJoin(fs.workingDir, filename)
}

// insideWorkingDir checks whether filename is located within
// the fs.workingDir.
func (fs *OS) insideWorkingDir(filename string) (bool, error) {
	if filename == fs.workingDir {
		return true, nil
	}
	if !strings.HasPrefix(filename, fs.workingDir+string(filepath.Separator)) {
		return false, fmt.Errorf("path outside working dir")
	}
	return true, nil
}

// insideWorkingDirEval checks whether filename is contained within
// a dir that is within the fs.workingDir, by evaluating any symlinks
// that either filename or fs.workingDir may contain.
func (fs *OS) insideWorkingDirEval(filename string) (bool, error) {
	dir, err := filepath.EvalSymlinks(filepath.Dir(filename))
	if dir == "" || os.IsNotExist(err) {
		dir = filepath.Dir(filename)
	}
	wd, err := filepath.EvalSymlinks(fs.workingDir)
	if wd == "" || os.IsNotExist(err) {
		wd = fs.workingDir
	}
	if dir != wd && !strings.HasPrefix(dir, wd+string(filepath.Separator)) {
		return false, fmt.Errorf("path outside working dir")
	}
	return true, nil
}
