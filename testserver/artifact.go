/*
Copyright 2020 The Flux authors

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

package testserver

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha1"
	_ "crypto/sha256"
	_ "crypto/sha512"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	digest "github.com/opencontainers/go-digest"
)

// NewTempArtifactServer returns an ArtifactServer with a newly created temp
// dir as the artifact docroot.
func NewTempArtifactServer() (*ArtifactServer, error) {
	tmpDir, err := os.MkdirTemp("", "artifact-test-")
	if err != nil {
		return nil, err
	}
	server := NewHTTPServer(tmpDir)
	artifact := &ArtifactServer{server}
	return artifact, nil
}

// ArtifactServer is an HTTP/S artifact server for testing
// purposes. It offers utilities to generate mock tarball
// artifacts to be served by the server.
type ArtifactServer struct {
	*HTTPServer
}

// File holds the name and string contents of an artifact file.
type File struct {
	Name string
	Body string
}

// ArtifactFromFiles creates a tar.gz artifact from the given files and
// returns the file name of the artifact.
func (s *ArtifactServer) ArtifactFromFiles(files []File) (string, error) {
	fileName := calculateArtifactName(files)
	filePath := filepath.Join(s.docroot, fileName)
	gzFile, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer gzFile.Close()

	gw := gzip.NewWriter(gzFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, f := range files {
		hdr := &tar.Header{
			Name: f.Name,
			Mode: 0o600,
			Size: int64(len(f.Body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return "", err
		}
		if _, err := tw.Write([]byte(f.Body)); err != nil {
			return "", err
		}
	}
	return fileName, nil
}

// ArtifactFromDir creates a tar.gz artifact from the source directory into the destination dir
// and returns the artifact SHA256 checksum.
func (s *ArtifactServer) ArtifactFromDir(source, destination string) (string, error) {
	d, err := s.ArtifactFromDirWithDigest(source, destination, digest.SHA256)
	if err != nil {
		return "", err
	}
	return d.Encoded(), nil
}

// ArtifactFromDirWithDigest creates a tar.gz artifact using the given algorithm
// and returns its digest. The algorithm must be registered and available; callers
// can import github.com/opencontainers/go-digest/blake3 to register BLAKE3.
// The destination is relative to the server root.
func (s *ArtifactServer) ArtifactFromDirWithDigest(source, destination string, algorithm digest.Algorithm) (d digest.Digest, retErr error) {
	if !algorithm.Available() {
		return "", fmt.Errorf("%w: %s", digest.ErrDigestUnsupported, algorithm)
	}
	info, err := os.Stat(source)
	if err != nil {
		return "", fmt.Errorf("invalid source path %s: %w", source, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("invalid source path: %s", source)
	}
	root, err := os.OpenRoot(s.Root())
	if err != nil {
		return "", err
	}
	defer func() {
		retErr = errors.Join(retErr, root.Close())
		if retErr != nil {
			d = ""
		}
	}()
	f, err := root.Create(destination)
	if err != nil {
		return "", err
	}
	defer func() {
		retErr = errors.Join(retErr, f.Close())
		if retErr != nil {
			d = ""
			retErr = errors.Join(retErr, root.Remove(destination))
		}
	}()
	if err := f.Chmod(0o600); err != nil {
		return "", err
	}

	digester := algorithm.Digester()

	mw := io.MultiWriter(digester.Hash(), f)
	gw := gzip.NewWriter(mw)
	tw := tar.NewWriter(gw)

	retErr = filepath.Walk(source, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		if strings.HasPrefix(fi.Name(), ".") {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, p)
		if err != nil {
			return err
		}

		relFilePath := p
		if filepath.IsAbs(source) {
			relFilePath, err = filepath.Rel(source, p)
			if err != nil {
				return err
			}
		}
		header.Name = relFilePath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		f, err := os.Open(p)
		if err != nil {
			return err
		}
		_, err = io.Copy(tw, f)
		return errors.Join(err, f.Close())
	})
	retErr = errors.Join(retErr, tw.Close(), gw.Close())
	if retErr != nil {
		return "", retErr
	}

	return digester.Digest(), nil
}

// URLForFile returns the URL the given file can be reached at or
// an error if the server has not been started.
func (s *ArtifactServer) URLForFile(file string) (string, error) {
	if s.URL() == "" {
		return "", errors.New("server must be started to be able to determine the URL of the given file")
	}
	return fmt.Sprintf("%s/%s", s.URL(), file), nil
}

func calculateArtifactName(files []File) string {
	h := sha1.New()
	for _, f := range files {
		h.Write([]byte(f.Body))
	}
	return fmt.Sprintf("%x.tar.gz", h.Sum(nil))
}
