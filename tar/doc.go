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

// Package tar provides utilities for creating and extracting tar
// archives, with optional gzip compression. Tar writes a sanitized
// archive of a directory tree, skipping symlinks and other non-regular,
// non-directory entries. Untar safely extracts a tar archive into a
// target directory, rejecting path traversal and capping the total
// decompressed size.
//
// # Creating an archive
//
// Archive a directory tree to a file as a gzip-compressed tarball:
//
//	f, err := os.Create("archive.tar.gz")
//	if err != nil {
//		return err
//	}
//	defer f.Close()
//
//	if _, err := tar.Tar("/path/to/dir", f); err != nil {
//		return err
//	}
//
// Exclude entries with a filter and write a plain (non-gzipped) tar:
//
//	skipHidden := func(p string, fi os.FileInfo) bool {
//		return strings.HasPrefix(fi.Name(), ".")
//	}
//	_, err := tar.Tar("/path/to/dir", f,
//		tar.WithFilter(skipHidden),
//		tar.WithSkipGzip(),
//	)
//
// # Extracting an archive
//
// Extract a gzip-compressed tarball into a directory:
//
//	f, err := os.Open("archive.tar.gz")
//	if err != nil {
//		return err
//	}
//	defer f.Close()
//
//	if err := tar.Untar(f, "/path/to/target"); err != nil {
//		return err
//	}
//
// Raise the size limit and tolerate symlinks in the archive:
//
//	err := tar.Untar(f, "/path/to/target",
//		tar.WithMaxUntarSize(500<<20), // 500 MiB
//		tar.WithSkipSymlinks(),
//	)
package tar
