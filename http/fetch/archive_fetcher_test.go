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
	"fmt"
	"os"
	"path/filepath"
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
	artifactChecksum, err := testServer.ArtifactFromDir("testdata", artifactName)
	g.Expect(err).ToNot(HaveOccurred())

	tests := []struct {
		name            string
		url             string
		checksum        string
		maxDownloadSize int
		maxUntarSize    int
		wantErr         bool
		wantErrType     error
	}{
		{
			name:            "fetches and verifies the checksum",
			url:             artifactURL,
			checksum:        artifactChecksum,
			maxDownloadSize: -1,
			maxUntarSize:    -1,
			wantErr:         false,
		},
		{
			name:            "breaches max download size",
			url:             artifactURL,
			checksum:        artifactChecksum,
			maxDownloadSize: 1,
			maxUntarSize:    -1,
			wantErr:         true,
		},
		{
			name:            "breaches max untar size",
			url:             artifactURL,
			checksum:        artifactChecksum,
			maxDownloadSize: -1,
			maxUntarSize:    1,
			wantErr:         true,
		},
		{
			name:            "fails to verify the checksum",
			url:             artifactURL,
			checksum:        artifactChecksum + "1",
			maxDownloadSize: -1,
			maxUntarSize:    -1,
			wantErr:         true,
		},
		{
			name:            "fails with not found error",
			url:             artifactURL + "1",
			checksum:        artifactChecksum,
			maxDownloadSize: -1,
			maxUntarSize:    -1,
			wantErr:         true,
			wantErrType:     FileNotFoundError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			fetcher := NewArchiveFetcher(1, tt.maxDownloadSize, tt.maxUntarSize, "")
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
