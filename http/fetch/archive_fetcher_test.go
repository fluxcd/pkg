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

package fetch

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/testserver"
)

func TestArchiveFetcher_Fetch(t *testing.T) {
	g := NewWithT(t)
	tmpDir := t.TempDir()

	testServer, err := testserver.NewTempArtifactServer()
	if err != nil {
		g.Expect(err).NotTo(HaveOccurred(), "failed to create the test server")
	}
	fmt.Println("Starting the test server")
	testServer.Start()

	fileName := "testdata/manifests.yaml"
	artifactName := "manifests.tgz"
	artifactURL := fmt.Sprintf("%s/%s", testServer.URL(), artifactName)
	artifactChecksum, err := createArtifact(testServer, "testdata", artifactName)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name        string
		url         string
		checksum    string
		wantErr     bool
		wantErrType error
	}{
		{
			name:     "fetches and verifies the checksum",
			url:      artifactURL,
			checksum: artifactChecksum,
			wantErr:  false,
		},
		{
			name:     "fails to verify the checksum",
			url:      artifactURL,
			checksum: artifactChecksum + "1",
			wantErr:  true,
		},
		{
			name:        "fails with not found error",
			url:         artifactURL + "1",
			checksum:    artifactChecksum,
			wantErr:     true,
			wantErrType: FileNotFoundError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			fetcher := NewArchiveFetcher(1, -1, "")
			err = fetcher.Fetch(tt.url, tt.checksum, tmpDir)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tt.wantErrType != nil {
					g.Expect(err).To(Equal(tt.wantErrType))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())

				originContent, err := os.ReadFile(fileName)
				g.Expect(err).ToNot(HaveOccurred())

				fetchedContent, err := os.ReadFile(filepath.Join(tmpDir, fileName))
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(string(originContent)).To(BeIdenticalTo(string(fetchedContent)))
			}
		})
	}

}

func createArtifact(artifactServer *testserver.ArtifactServer, source, destination string) (string, error) {
	if f, err := os.Stat(source); os.IsNotExist(err) || !f.IsDir() {
		return "", fmt.Errorf("invalid source path: %s", source)
	}
	f, err := os.Create(filepath.Join(artifactServer.Root(), destination))
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

		// Ignore anything that is not a file (directories, symlinks)
		if !fi.Mode().IsRegular() {
			return nil
		}

		// Ignore dotfiles
		if strings.HasPrefix(fi.Name(), ".") {
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
