/*
Copyright 2022 The Flux authors

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

package oci

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/fluxcd/pkg/oci/internal/fs"
	"github.com/fluxcd/pkg/sourceignore"
	"github.com/fluxcd/pkg/tar"
)

// Build archives the given directory as a tarball to the given local path.
// While archiving, any environment specific data (for example, the user and group name) is stripped from file headers.
func (c *Client) Build(artifactPath, sourceDir string, ignorePaths []string) (err error) {
	return build(artifactPath, sourceDir, ignorePaths)
}

func build(artifactPath, sourceDir string, ignorePaths []string) (err error) {
	absSrc, err := filepath.Abs(sourceDir)
	if err != nil {
		return err
	}

	srcInfo, err := os.Stat(absSrc)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("source path does not exist: %s", absSrc)
		}
		return fmt.Errorf("invalid source path %s: %w", absSrc, err)
	}

	tf, err := os.CreateTemp(filepath.Split(absSrc))
	if err != nil {
		return err
	}
	tmpName := tf.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpName)
		}
	}()

	// If the source is a single file, stage it in a temp dir so Tar can
	// archive it as a directory tree containing that one entry.
	tarDir := absSrc
	if !srcInfo.IsDir() {
		stage, stageErr := os.MkdirTemp("", "oci-build-")
		if stageErr != nil {
			tf.Close()
			return stageErr
		}
		defer os.RemoveAll(stage)

		if err := copyFileContents(filepath.Join(stage, srcInfo.Name()), absSrc, srcInfo.Mode()); err != nil {
			tf.Close()
			return err
		}
		tarDir = stage
	}

	ignore := strings.Join(ignorePaths, "\n")
	domain := strings.Split(filepath.Clean(tarDir), string(filepath.Separator))
	ps := sourceignore.ReadPatterns(strings.NewReader(ignore), domain)
	matcher := sourceignore.NewMatcher(ps)
	filter := func(p string, fi os.FileInfo) bool {
		return matcher.Match(strings.Split(p, string(filepath.Separator)), fi.IsDir())
	}

	if _, err := tar.Tar(tarDir, tf, tar.WithFilter(filter)); err != nil {
		tf.Close()
		return err
	}
	if err := tf.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o640); err != nil {
		return err
	}
	return fs.RenameWithFallback(tmpName, artifactPath)
}

func copyFileContents(dst, src string, mode os.FileMode) (err error) {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sf.Close()
	df, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode.Perm())
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := df.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(df, sf)
	return err
}
