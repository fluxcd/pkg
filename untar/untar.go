// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2020 The FluxCD contributors. All rights reserved.
// Adapted from: golang.org/x/build/internal/untar

// Package untar untars a tarball to disk.
package untar

import (
	"io"

	"github.com/fluxcd/pkg/tar"
)

// Untar reads the gzip-compressed tar file from r and writes it into dir.
func Untar(r io.Reader, dir string) (summary string, err error) {
	return "", tar.Untar(r, dir, tar.WithMaxUntarSize(-1))
}
