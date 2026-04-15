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
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// Tar writes a tar archive of dir to w and returns the number of bytes
// written.
//
// By default, the archive is gzip-compressed; use WithSkipGzip to write
// a plain tar stream. Use WithFilter to exclude entries by path or
// FileInfo. The directory tree is walked recursively; symlinks and
// other non-regular, non-directory entries are silently skipped.
// Headers are sanitized to produce reproducible archives: uid, gid,
// user and group names, and all timestamps are zeroed.
func Tar(dir string, w io.Writer, opts ...Option) (int64, error) {
	var o tarOpts
	o.applyOpts(opts...)

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return 0, err
	}

	if fi, err := os.Stat(absDir); err != nil {
		return 0, fmt.Errorf("invalid dir path %s: %w", absDir, err)
	} else if !fi.IsDir() {
		return 0, fmt.Errorf("not a directory: %s", absDir)
	}

	cw := &countWriter{w: w}

	var gw *gzip.Writer
	var tw *tar.Writer
	if o.skipGzip {
		tw = tar.NewWriter(cw)
	} else {
		gw = gzip.NewWriter(cw)
		tw = tar.NewWriter(gw)
	}

	buf := make([]byte, bufferSize)
	if err := filepath.Walk(absDir, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks and other non-regular, non-directory entries.
		if m := fi.Mode(); !(m.IsRegular() || m.IsDir()) {
			return nil
		}

		if o.filter != nil && o.filter(p, fi) {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, p)
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(absDir, p)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)

		// Sanitize environment-specific data.
		header.Gid = 0
		header.Uid = 0
		header.Uname = ""
		header.Gname = ""
		header.ModTime = time.Time{}
		header.AccessTime = time.Time{}
		header.ChangeTime = time.Time{}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(p)
		if err != nil {
			return err
		}
		_, err = copyBuffer(tw, f, buf)
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		return err
	}); err != nil {
		_ = tw.Close()
		if gw != nil {
			_ = gw.Close()
		}
		return cw.n, err
	}

	if err := tw.Close(); err != nil {
		if gw != nil {
			_ = gw.Close()
		}
		return cw.n, err
	}
	if gw != nil {
		if err := gw.Close(); err != nil {
			return cw.n, err
		}
	}

	return cw.n, nil
}

// countWriter wraps an io.Writer and counts the bytes written.
type countWriter struct {
	w io.Writer
	n int64
}

func (cw *countWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}
