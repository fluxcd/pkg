// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2020 The Flux authors. All rights reserved.
// Adapted from: golang.org/x/build/internal/untar

package tar

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
)

const (
	// DefaultMaxUntarSize defines the default (100MB) max amount of bytes that Untar will process.
	DefaultMaxUntarSize = 100 << (10 * 2)

	// UnlimitedUntarSize defines the value which disables untar size checks for maxUntarSize.
	UnlimitedUntarSize = -1

	// bufferSize defines the size of the buffer used when copying the tar file entries.
	bufferSize = 32 * 1024
)

// Untar extracts a tar archive read from r into dir.
//
// By default, r is expected to be gzip-compressed; use WithSkipGzip to
// read a plain tar stream. Extraction is capped at DefaultMaxUntarSize
// bytes; use WithMaxUntarSize to raise, lower, or disable the limit.
// Use WithFilter to skip entries by name or FileInfo during extraction.
// Entries with paths that escape dir are rejected. Symlinks fail
// extraction unless WithSkipSymlinks is set, in which case they are
// silently dropped.
//
// If dir is a relative path, it cannot ascend from the current working
// directory. If dir exists, it must be a directory; otherwise it is
// created.
func Untar(r io.Reader, dir string, inOpts ...Option) error {
	opts := tarOpts{
		maxUntarSize: DefaultMaxUntarSize,
	}
	opts.applyOpts(inOpts...)

	dir = filepath.Clean(dir)
	if !filepath.IsAbs(dir) {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}

		if dir, err = securejoin.SecureJoin(cwd, dir); err != nil {
			return err
		}
	}

	fi, err := os.Lstat(dir)
	// Dir does not need to exist, as it can later be created.
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot lstat '%s': %w", dir, err)
	}

	if err == nil && !fi.IsDir() {
		return fmt.Errorf("dir '%s' must be a directory", dir)
	}

	madeDir := map[string]bool{}

	var rc = io.NopCloser(r)
	if !opts.skipGzip {
		var err error
		rc, err = gzip.NewReader(r)
		if err != nil {
			return fmt.Errorf("requires gzip-compressed body: %w", err)
		}
	}
	tr := tar.NewReader(rc)

	var processedBytes int64
	t0 := time.Now()

	// Reuse a single buffer for all file copies.
	buf := make([]byte, bufferSize)
	for {
		f, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("tar error: %w", err)
		}
		processedBytes += f.Size
		if opts.maxUntarSize > UnlimitedUntarSize &&
			processedBytes > int64(opts.maxUntarSize) {
			return fmt.Errorf("tar %q is bigger than max archive size of %d bytes", f.Name, opts.maxUntarSize)
		}
		if !validRelPath(f.Name) {
			return fmt.Errorf("tar contained invalid name error %q", f.Name)
		}
		rel := filepath.FromSlash(f.Name)
		abs := filepath.Join(dir, rel)

		fi := f.FileInfo()
		mode := fi.Mode()

		if opts.filter != nil && opts.filter(f.Name, fi) {
			continue
		}

		switch {
		case mode.IsRegular():
			// Make the directory. This is redundant because it should
			// already be made by a directory entry in the tar
			// beforehand. Thus, don't check for errors; the next
			// write will fail with the same error.
			parentDir := filepath.Dir(abs)
			if !madeDir[parentDir] {
				if err := os.MkdirAll(parentDir, 0o750); err != nil {
					return err
				}
				madeDir[parentDir] = true
			}
			if runtime.GOOS == "darwin" && mode&0111 != 0 {
				// The darwin kernel caches binary signatures
				// and SIGKILLs binaries with mismatched
				// signatures. Overwriting a binary with
				// O_TRUNC does not clear the cache, rendering
				// the new copy unusable. Removing the original
				// file first does clear the cache. See #54132.
				err := os.Remove(abs)
				if err != nil && !errors.Is(err, fs.ErrNotExist) {
					return err
				}
			}
			wf, err := os.OpenFile(abs, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
			if err != nil {
				return err
			}

			n, err := copyBuffer(wf, tr, buf)
			if err != nil && !errors.Is(err, io.EOF) {
				return fmt.Errorf("error copying buffer: %w", err)
			}

			if closeErr := wf.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
			if err != nil {
				return fmt.Errorf("error writing to %s: %w", abs, err)
			}
			if n != f.Size {
				return fmt.Errorf("only wrote %d bytes to %s; expected %d", n, abs, f.Size)
			}
			modTime := f.ModTime
			if modTime.After(t0) {
				// Ensures that that files extracted are not newer then the
				// current system time.
				modTime = t0
			}
			if !modTime.IsZero() {
				if err = os.Chtimes(abs, modTime, modTime); err != nil {
					return fmt.Errorf("error changing file time %s: %w", abs, err)
				}
			}
		case mode.IsDir():
			// Ensure the owner can always traverse, read, and write
			// into extracted directories, regardless of what the tar
			// header claims. This prevents crafted archives from
			// creating directories that block cleanup or future writes.
			dirPerm := mode.Perm() | 0o700
			if err := os.MkdirAll(abs, dirPerm); err != nil {
				return err
			}
			madeDir[abs] = true
		case mode&os.ModeSymlink == os.ModeSymlink:
			if !opts.skipSymlinks {
				return fmt.Errorf("tar file entry %s is a symlink, which is not allowed in this context", f.Name)
			}
		default:
			return fmt.Errorf("tar file entry %s contained unsupported file type %v", f.Name, mode)
		}
	}
	return rc.Close()
}

// Uses a variant of io.CopyBuffer which ensures that a buffer is being used.
// The upstream version prioritises the use of interfaces WriterTo and ReadFrom
// which in this case causes the entirety of the tar file entry to be loaded
// into memory.
//
// Original source:
// https://github.com/golang/go/blob/6f445a9db55f65e55c5be29d3c506ecf3be37915/src/io/io.go#L405
func copyBuffer(dst io.Writer, src io.Reader, buf []byte) (written int64, err error) {
	if buf == nil {
		return 0, fmt.Errorf("buf is nil")
	}
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[:nr])
			// Guard against a broken Writer: negative byte count
			// or claiming more bytes written than provided.
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = errors.New("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
		}
		if er != nil {
			if !errors.Is(er, io.EOF) {
				err = er
			}
			break
		}
	}
	return written, err
}

func validRelPath(p string) bool {
	if p == "" || strings.Contains(p, `\`) || strings.HasPrefix(p, "/") || strings.Contains(p, "../") {
		return false
	}
	return true
}
