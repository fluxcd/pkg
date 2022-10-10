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
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
			Mode: 0600,
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
	if f, err := os.Stat(source); os.IsNotExist(err) || !f.IsDir() {
		return "", fmt.Errorf("invalid source path: %s", source)
	}
	f, err := os.Create(filepath.Join(s.Root(), destination))
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			os.Remove(f.Name())
		}
	}()

	h := sha256.New()

	mw := io.MultiWriter(h, f)
	gw := gzip.NewWriter(mw)
	tw := tar.NewWriter(gw)

	if err = filepath.Walk(source, func(p string, fi os.FileInfo, err error) error {
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
			f.Close()
			return err
		}
		if _, err := io.Copy(tw, f); err != nil {
			f.Close()
			return err
		}
		return f.Close()
	}); err != nil {
		return "", err
	}

	if err := tw.Close(); err != nil {
		gw.Close()
		f.Close()
		return "", err
	}
	if err := gw.Close(); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	if err := os.Chmod(f.Name(), 0644); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
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
