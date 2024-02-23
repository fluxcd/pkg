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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/tar"
	"github.com/fluxcd/pkg/testserver"
)

func TestArchiveFetcher_FetchWithContext(t *testing.T) {
	g := NewWithT(t)
	tmpDir := t.TempDir()

	testServer, err := testserver.NewTempArtifactServer()
	if err != nil {
		g.Expect(err).NotTo(HaveOccurred(), "failed to create the test server")
	}
	fmt.Println("Starting the test server")
	testServer.Start()

	manifestsFileName := "testdata/manifests.yaml"
	artifactName := "manifests.tgz"
	artifactURL := fmt.Sprintf("%s/%s", testServer.URL(), artifactName)
	artifactChecksum, err := testServer.ArtifactFromDir("testdata", artifactName)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name               string
		url                string
		digest             string
		originContentPath  string
		fetchedContentPath string
		opts               []Option
		wantErr            bool
		wantErrType        error
	}{
		{
			name:               "fetches and verifies the digest",
			url:                artifactURL,
			digest:             "sha256:" + artifactChecksum,
			originContentPath:  manifestsFileName,
			fetchedContentPath: filepath.Join(tmpDir, manifestsFileName),
			opts:               []Option{WithUntar()},
			wantErr:            false,
		},
		{
			name:               "fetches and verifies the digest without extracting",
			url:                artifactURL,
			digest:             "sha256:" + artifactChecksum,
			originContentPath:  filepath.Join(testServer.Root(), artifactName),
			fetchedContentPath: filepath.Join(tmpDir, artifactName),
			opts:               []Option{},
			wantErr:            false,
		},
		{
			name:               "fetches and verifies the digest without extracting, with file name",
			url:                artifactURL,
			digest:             "sha256:" + artifactChecksum,
			originContentPath:  filepath.Join(testServer.Root(), artifactName),
			fetchedContentPath: filepath.Join(tmpDir, "tarball.tar.gz"),
			opts:               []Option{WithFileName("tarball.tar.gz")},
			wantErr:            false,
		},
		{
			name:               "fetches and verifies the checksum",
			url:                artifactURL,
			digest:             artifactChecksum,
			originContentPath:  manifestsFileName,
			fetchedContentPath: filepath.Join(tmpDir, manifestsFileName),
			opts:               []Option{WithUntar()},
			wantErr:            false,
		},
		{
			name:               "breaches max download size",
			url:                artifactURL,
			digest:             artifactChecksum,
			originContentPath:  manifestsFileName,
			fetchedContentPath: filepath.Join(tmpDir, manifestsFileName),
			opts:               []Option{WithUntar(), WithMaxDownloadSize(1)},
			wantErr:            true,
		},
		{
			name:               "breaches max untar size",
			url:                artifactURL,
			digest:             artifactChecksum,
			originContentPath:  manifestsFileName,
			fetchedContentPath: filepath.Join(tmpDir, manifestsFileName),
			opts:               []Option{WithUntar(tar.WithMaxUntarSize(1))},
			wantErr:            true,
		},
		{
			name:               "fails with empty digest error",
			url:                artifactURL,
			digest:             "",
			originContentPath:  manifestsFileName,
			fetchedContentPath: filepath.Join(tmpDir, manifestsFileName),
			opts:               []Option{WithUntar()},
			wantErr:            true,
		},
		{
			name:               "fails with digest parsing error",
			url:                artifactURL,
			digest:             "sha1:4c624b40731c876b23d36fe732833ea8261f7f00",
			originContentPath:  manifestsFileName,
			fetchedContentPath: filepath.Join(tmpDir, manifestsFileName),
			opts:               []Option{WithUntar()},
			wantErr:            true,
		},
		{
			name:               "fails to verify the digest",
			url:                artifactURL,
			digest:             "sha256:5c234ee52ff0e3dcc8528d6b9383cc235ad13a11658466f29df3be9eda6ee447",
			originContentPath:  manifestsFileName,
			fetchedContentPath: filepath.Join(tmpDir, manifestsFileName),
			opts:               []Option{WithUntar()},
			wantErr:            true,
		},
		{
			name:               "fails with not found error",
			url:                artifactURL + "1",
			digest:             artifactChecksum,
			originContentPath:  manifestsFileName,
			fetchedContentPath: filepath.Join(tmpDir, manifestsFileName),
			opts:               []Option{WithUntar()},
			wantErr:            true,
			wantErrType:        ErrFileNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			fetcher := New(tt.opts...)
			err = fetcher.FetchWithContext(context.Background(), tt.url, tt.digest, tmpDir)

			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tt.wantErrType != nil {
					g.Expect(err).To(Equal(tt.wantErrType))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())

				originContent, err := os.ReadFile(tt.originContentPath)
				g.Expect(err).ToNot(HaveOccurred())

				fetchedContent, err := os.ReadFile(tt.fetchedContentPath)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(string(originContent)).To(BeIdenticalTo(string(fetchedContent)))
			}
		})
	}
}

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
	artifactChecksum, err := testServer.ArtifactFromDir("testdata", artifactName)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name            string
		url             string
		digest          string
		maxDownloadSize int
		maxUntarSize    int
		wantErr         bool
		wantErrType     error
	}{
		{
			name:            "fetches and verifies the digest",
			url:             artifactURL,
			digest:          "sha256:" + artifactChecksum,
			maxDownloadSize: -1,
			maxUntarSize:    -1,
			wantErr:         false,
		},
		{
			name:            "fetches and verifies the checksum",
			url:             artifactURL,
			digest:          artifactChecksum,
			maxDownloadSize: -1,
			maxUntarSize:    -1,
			wantErr:         false,
		},
		{
			name:            "breaches max download size",
			url:             artifactURL,
			digest:          artifactChecksum,
			maxDownloadSize: 1,
			maxUntarSize:    -1,
			wantErr:         true,
		},
		{
			name:            "breaches max untar size",
			url:             artifactURL,
			digest:          artifactChecksum,
			maxDownloadSize: -1,
			maxUntarSize:    1,
			wantErr:         true,
		},
		{
			name:            "fails with empty digest error",
			url:             artifactURL,
			digest:          "",
			maxDownloadSize: -1,
			maxUntarSize:    -1,
			wantErr:         true,
		},
		{
			name:            "fails with digest parsing error",
			url:             artifactURL,
			digest:          "sha1:4c624b40731c876b23d36fe732833ea8261f7f00",
			maxDownloadSize: -1,
			maxUntarSize:    1,
			wantErr:         true,
		},
		{
			name:            "fails to verify the digest",
			url:             artifactURL,
			digest:          "sha256:5c234ee52ff0e3dcc8528d6b9383cc235ad13a11658466f29df3be9eda6ee447",
			maxDownloadSize: -1,
			maxUntarSize:    -1,
			wantErr:         true,
		},
		{
			name:            "fails with not found error",
			url:             artifactURL + "1",
			digest:          artifactChecksum,
			maxDownloadSize: -1,
			maxUntarSize:    -1,
			wantErr:         true,
			wantErrType:     ErrFileNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			fetcher := NewArchiveFetcher(1, tt.maxDownloadSize, tt.maxUntarSize, "")
			err = fetcher.Fetch(tt.url, tt.digest, tmpDir)

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
