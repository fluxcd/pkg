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

import "os"

// Option configures the behavior of Tar and Untar. Options are
// silently ignored by operations they do not apply to.
type Option func(*tarOpts)

type tarOpts struct {
	// maxUntarSize represents the limit size (bytes) for archives being decompressed by Untar.
	// When max is a negative value the size checks are disabled.
	maxUntarSize int

	// skipSymlinks ignores symlinks instead of failing the decompression.
	skipSymlinks bool

	// skipGzip disables gzip compression: Tar writes a plain tar stream,
	// and Untar reads one.
	skipGzip bool

	// filter is called for each entry during archiving or extraction.
	// If it returns true, the entry is excluded.
	filter func(path string, fi os.FileInfo) bool
}

// WithMaxUntarSize sets the limit size for archives being decompressed by Untar.
// When max is equal or less than 0 disables size checks.
func WithMaxUntarSize(max int) Option {
	return func(t *tarOpts) {
		t.maxUntarSize = max
	}
}

// WithSkipSymlinks allows for symlinks to be present
// in the tarball and skips them when decompressing.
func WithSkipSymlinks() Option {
	return func(t *tarOpts) {
		t.skipSymlinks = true
	}
}

// WithSkipGzip disables gzip compression: Tar writes a plain tar stream,
// and Untar reads one.
func WithSkipGzip() Option {
	return func(t *tarOpts) {
		t.skipGzip = true
	}
}

// WithFilter sets a predicate called for each entry during archiving
// or extraction. Entries for which fn returns true are excluded. During
// Tar the path is the absolute filesystem path; during Untar it is the
// slash-separated name from the tar header.
func WithFilter(fn func(path string, fi os.FileInfo) bool) Option {
	return func(t *tarOpts) {
		t.filter = fn
	}
}

// applyOpts applies the given Option to t.
func (t *tarOpts) applyOpts(opts ...Option) {
	for _, opt := range opts {
		opt(t)
	}
}
