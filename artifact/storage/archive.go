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

package storage

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/oci"
	"github.com/fluxcd/pkg/sourceignore"

	intdigest "github.com/fluxcd/pkg/artifact/digest"
)

const (
	// DefaultFileMode is the permission mode applied to files inside an artifact archive.
	DefaultFileMode int64 = 0o600
	// DefaultDirMode is the permission mode applied to all directories inside an artifact archive.
	DefaultDirMode int64 = 0o750
	// DefaultExeFileMode is the permission mode applied to executable files inside an artifact archive.
	DefaultExeFileMode int64 = 0o700
)

// writeCounter is an implementation of io.Writer
// that only records the number of bytes written.
type writeCounter struct {
	written int64
}

// Write implements the io.Writer interface.
func (wc *writeCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.written += int64(n)
	return n, nil
}

// ArchiveFileFilter must return true if a file should not be included
// in the archive after inspecting the given path and/or os.FileInfo.
type ArchiveFileFilter func(p string, fi os.FileInfo) bool

// SourceIgnoreFilter returns an ArchiveFileFilter that filters out files matching
// sourceignore.VCSPatterns and any of the provided patterns.
// If an empty gitignore.Pattern slice is given, the matcher is set to sourceignore.NewDefaultMatcher.
func SourceIgnoreFilter(ps []gitignore.Pattern, domain []string) ArchiveFileFilter {
	matcher := sourceignore.NewDefaultMatcher(ps, domain)
	if len(ps) > 0 {
		ps = append(sourceignore.VCSPatterns(domain), ps...)
		matcher = sourceignore.NewMatcher(ps)
	}
	return func(p string, fi os.FileInfo) bool {
		return matcher.Match(strings.Split(p, string(filepath.Separator)), fi.IsDir())
	}
}

// Archive atomically archives the given directory as a tarball to the given meta.Artifact path,
// excluding directories and any ArchiveFileFilter matches. While archiving, any environment
// specific data (for example, the user and group name) is stripped from file headers.
// If successful, it sets the digest and last update time on the artifact.
func (s Storage) Archive(artifact *meta.Artifact, dir string, filter ArchiveFileFilter) (err error) {
	if f, err := os.Stat(dir); os.IsNotExist(err) || !f.IsDir() {
		return fmt.Errorf("invalid dir path: %s", dir)
	}

	localPath := s.LocalPath(*artifact)
	tf, err := os.CreateTemp(filepath.Split(localPath))
	if err != nil {
		return err
	}
	tmpName := tf.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpName)
		}
	}()

	d := intdigest.Canonical.Digester()
	sz := &writeCounter{}
	mw := io.MultiWriter(d.Hash(), tf, sz)

	gw := gzip.NewWriter(mw)
	tw := tar.NewWriter(gw)
	if err := filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ignore anything that is not a file or directories e.g. symlinks
		if m := fi.Mode(); !(m.IsRegular() || m.IsDir()) {
			return nil
		}

		// Skip filtered files
		if filter != nil && filter(p, fi) {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, p)
		if err != nil {
			return err
		}

		// The name needs to be modified to maintain directory structure
		// as tar.FileInfoHeader only has access to the base name of the file.
		// Ref: https://golang.org/src/archive/tar/common.go?#L626
		relFilePath := p
		if filepath.IsAbs(dir) {
			relFilePath, err = filepath.Rel(dir, p)
			if err != nil {
				return err
			}
		}
		sanitizeHeader(relFilePath, header)

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			f.Close()
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}); err != nil {
		tw.Close()
		gw.Close()
		tf.Close()
		return err
	}

	if err := tw.Close(); err != nil {
		gw.Close()
		tf.Close()
		return err
	}
	if err := gw.Close(); err != nil {
		tf.Close()
		return err
	}
	if err := tf.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tmpName, 0o600); err != nil {
		return err
	}

	if err := oci.RenameWithFallback(tmpName, localPath); err != nil {
		return err
	}

	artifact.Digest = d.Digest().String()
	artifact.LastUpdateTime = metav1.Now()
	artifact.Size = &sz.written

	return nil
}

// sanitizeHeader modifies the tar.Header to be relative to the root of the
// archive and removes any environment specific data.
func sanitizeHeader(relP string, h *tar.Header) {
	// Modify the name to be relative to the root of the archive,
	// this ensures we maintain the same structure when extracting.
	h.Name = relP

	// We want to remove any environment specific data as well, this
	// ensures the checksum is purely content based.
	h.Gid = 0
	h.Uid = 0
	h.Uname = ""
	h.Gname = ""
	h.ModTime = time.Time{}
	h.AccessTime = time.Time{}
	h.ChangeTime = time.Time{}

	// Override the mode to be the default for the type of file.
	setDefaultMode(h)
}

// setDefaultMode sets the default mode for the given header.
func setDefaultMode(h *tar.Header) {
	if h.FileInfo().IsDir() {
		h.Mode = DefaultDirMode
		return
	}

	if h.FileInfo().Mode().IsRegular() {
		mode := h.FileInfo().Mode()
		if mode&os.ModeType == 0 && mode&0o111 != 0 {
			h.Mode = DefaultExeFileMode
			return
		}
		h.Mode = DefaultFileMode
		return
	}
}
