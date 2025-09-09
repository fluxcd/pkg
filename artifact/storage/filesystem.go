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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
	intdigest "github.com/fluxcd/pkg/artifact/digest"
	"github.com/fluxcd/pkg/oci"
	pkgtar "github.com/fluxcd/pkg/tar"
)

// AtomicWriteFile atomically writes the io.Reader contents to the meta.Artifact path.
// If successful, it sets the digest and last update time on the artifact.
func (s Storage) AtomicWriteFile(artifact *meta.Artifact, reader io.Reader, mode os.FileMode) (err error) {
	localPath := s.LocalPath(*artifact)
	tf, err := os.CreateTemp(filepath.Split(localPath))
	if err != nil {
		return err
	}
	tfName := tf.Name()
	defer func() {
		if err != nil {
			os.Remove(tfName)
		}
	}()

	d := intdigest.Canonical.Digester()
	sz := &writeCounter{}
	mw := io.MultiWriter(tf, d.Hash(), sz)

	if _, err := io.Copy(mw, reader); err != nil {
		tf.Close()
		return err
	}
	if err := tf.Close(); err != nil {
		return err
	}

	if err := os.Chmod(tfName, mode); err != nil {
		return err
	}

	if err := oci.RenameWithFallback(tfName, localPath); err != nil {
		return err
	}

	artifact.Digest = d.Digest().String()
	artifact.LastUpdateTime = metav1.Now()
	artifact.Size = &sz.written

	return nil
}

// Copy atomically copies the io.Reader contents to the meta.Artifact path.
// If successful, it sets the digest and last update time on the artifact.
func (s Storage) Copy(artifact *meta.Artifact, reader io.Reader) (err error) {
	localPath := s.LocalPath(*artifact)
	tf, err := os.CreateTemp(filepath.Split(localPath))
	if err != nil {
		return err
	}
	tfName := tf.Name()
	defer func() {
		if err != nil {
			os.Remove(tfName)
		}
	}()

	d := intdigest.Canonical.Digester()
	sz := &writeCounter{}
	mw := io.MultiWriter(tf, d.Hash(), sz)

	if _, err := io.Copy(mw, reader); err != nil {
		tf.Close()
		return err
	}
	if err := tf.Close(); err != nil {
		return err
	}

	if err := oci.RenameWithFallback(tfName, localPath); err != nil {
		return err
	}

	artifact.Digest = d.Digest().String()
	artifact.LastUpdateTime = metav1.Now()
	artifact.Size = &sz.written

	return nil
}

// CopyFromPath atomically copies the contents of the given path to the path of the meta.Artifact.
// If successful, the digest and last update time on the artifact is set.
func (s Storage) CopyFromPath(artifact *meta.Artifact, path string) (err error) {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	err = s.Copy(artifact, f)
	return err
}

// MkdirAll calls os.MkdirAll for the given meta.Artifact base dir.
func (s Storage) MkdirAll(artifact meta.Artifact) error {
	dir := filepath.Dir(s.LocalPath(artifact))
	return os.MkdirAll(dir, 0o700)
}

// Remove calls os.Remove for the given meta.Artifact path.
func (s Storage) Remove(artifact meta.Artifact) error {
	return os.Remove(s.LocalPath(artifact))
}

// RemoveAll calls os.RemoveAll for the given meta.Artifact base dir.
func (s Storage) RemoveAll(artifact meta.Artifact) (string, error) {
	var deletedDir string
	dir := filepath.Dir(s.LocalPath(artifact))
	// Check if the dir exists.
	_, err := os.Stat(dir)
	if err == nil {
		deletedDir = dir
	}
	return deletedDir, os.RemoveAll(dir)
}

// RemoveAllButCurrent removes all files for the given meta.Artifact base dir, excluding the current one.
func (s Storage) RemoveAllButCurrent(artifact meta.Artifact) ([]string, error) {
	deletedFiles := []string{}
	localPath := s.LocalPath(artifact)
	dir := filepath.Dir(localPath)
	var errors []string
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			errors = append(errors, err.Error())
			return nil
		}

		if path != localPath && !info.IsDir() && info.Mode()&os.ModeSymlink != os.ModeSymlink {
			if err := os.Remove(path); err != nil {
				errors = append(errors, info.Name())
			} else {
				// Collect the successfully deleted file paths.
				deletedFiles = append(deletedFiles, path)
			}
		}
		return nil
	})

	if len(errors) > 0 {
		return deletedFiles, fmt.Errorf("failed to remove files: %s", strings.Join(errors, " "))
	}
	return deletedFiles, nil
}

// CopyToPath copies the contents in the (sub)path of the given artifact to the given path.
func (s Storage) CopyToPath(artifact *meta.Artifact, subPath, toPath string) error {
	// create a tmp directory to store artifact
	tmp, err := os.MkdirTemp("", "flux-include-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	// read artifact file content
	localPath := s.LocalPath(*artifact)
	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	// untar the artifact
	untarPath := filepath.Join(tmp, "unpack")
	if err = pkgtar.Untar(f, untarPath, pkgtar.WithMaxUntarSize(-1)); err != nil {
		return err
	}

	// create the destination parent dir
	if err = os.MkdirAll(filepath.Dir(toPath), os.ModePerm); err != nil {
		return err
	}

	// copy the artifact content to the destination dir
	fromPath, err := securejoin.SecureJoin(untarPath, subPath)
	if err != nil {
		return err
	}
	if err := oci.RenameWithFallback(fromPath, toPath); err != nil {
		return err
	}
	return nil
}

// Symlink creates or updates a symbolic link for the given meta.Artifact and returns the URL for the symlink.
func (s Storage) Symlink(artifact meta.Artifact, linkName string) (string, error) {
	localPath := s.LocalPath(artifact)
	dir := filepath.Dir(localPath)
	link := filepath.Join(dir, linkName)
	tmpLink := link + ".tmp"

	if err := os.Remove(tmpLink); err != nil && !os.IsNotExist(err) {
		return "", err
	}

	if err := os.Symlink(localPath, tmpLink); err != nil {
		return "", err
	}

	if err := os.Rename(tmpLink, link); err != nil {
		return "", err
	}

	return fmt.Sprintf("http://%s/%s", s.Hostname, filepath.Join(filepath.Dir(artifact.Path), linkName)), nil
}
