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

package client

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/types"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/oci"
)

func Test_Push_Pull(t *testing.T) {
	ctx := context.Background()
	c := NewClient(DefaultOptions())
	source := "github.com/fluxcd/flux2"
	revision := "rev"
	repo := "test-push" + randStringRunes(5)
	ct := time.Now().UTC()
	created := ct.Format(time.RFC3339)

	tests := []struct {
		name              string
		sourcePath        string
		tag               string
		ignorePaths       []string
		opts              []PushOption
		expectErr         bool
		expectedMediaType types.MediaType
	}{
		{
			name:              "push directory (default layer type)",
			tag:               "v0.0.1",
			sourcePath:        "testdata/artifact",
			expectedMediaType: oci.CanonicalContentMediaType,
		},
		{
			name:              "push directory (specify layer type)",
			tag:               "v0.0.1",
			sourcePath:        "testdata/artifact",
			expectedMediaType: oci.CanonicalContentMediaType,
			opts: []PushOption{
				WithPushLayerType(LayerTypeTarball),
			},
		},
		{
			name:       "push static file",
			tag:        "v0.0.2",
			sourcePath: "testdata/artifact/deployment.yaml",
			opts: []PushOption{
				WithPushLayerType(LayerTypeStatic),
				WithPushMediaTypeExt("ml"),
			},
			expectedMediaType: getLayerMediaType("ml"),
		},
		{
			name:       "push directory as static layer (should return error)",
			sourcePath: "testdata/artifact",
			opts: []PushOption{
				WithPushLayerType(LayerTypeStatic),
			},
			expectErr: true,
		},
		{
			name:       "push static file without media type extension",
			tag:        "v0.0.2",
			sourcePath: "testdata/artifact/deployment.yaml",
			opts: []PushOption{
				WithPushLayerType(LayerTypeStatic),
			},
			expectedMediaType: oci.CanonicalMediaTypePrefix,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			url := fmt.Sprintf("%s/%s:%s", dockerReg, repo, tt.tag)
			metadata := Metadata{
				Source:   source,
				Revision: revision,
				Created:  created,
				Annotations: map[string]string{
					"org.opencontainers.image.documentation": "https://my/readme.md",
					"org.opencontainers.image.licenses":      "Apache-2.0",
				},
			}
			opts := append(tt.opts, WithPushMetadata(metadata))

			// Build and push the artifact to registry
			_, err := c.Push(ctx, url, tt.sourcePath, opts...)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).To(Not(HaveOccurred()))
			// Verify that the artifact and its tag is present in the registry
			tags, err := crane.ListTags(fmt.Sprintf("%s/%s", dockerReg, repo))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(tags).To(ContainElement(tt.tag))

			// Pull the artifact from registry
			image, err := crane.Pull(fmt.Sprintf("%s/%s:%s", dockerReg, repo, tt.tag))
			g.Expect(err).ToNot(HaveOccurred())

			// Extract the manifest from the pulled artifact
			manifest, err := image.Manifest()
			g.Expect(err).ToNot(HaveOccurred())

			// Verify that annotations exist in manifest
			g.Expect(manifest.Annotations[oci.CreatedAnnotation]).To(BeEquivalentTo(created))
			g.Expect(manifest.Annotations[oci.SourceAnnotation]).To(BeEquivalentTo(source))
			g.Expect(manifest.Annotations[oci.RevisionAnnotation]).To(BeEquivalentTo(revision))

			// Verify media types
			g.Expect(manifest.MediaType).To(Equal(types.OCIManifestSchema1))
			g.Expect(manifest.Config.MediaType).To(BeEquivalentTo(oci.CanonicalConfigMediaType))
			g.Expect(len(manifest.Layers)).To(BeEquivalentTo(1))
			g.Expect(manifest.Layers[0].MediaType).To(BeEquivalentTo(tt.expectedMediaType))

			// Verify custom annotations
			meta := MetadataFromAnnotations(manifest.Annotations)
			g.Expect(meta.Annotations["org.opencontainers.image.documentation"]).To(BeEquivalentTo("https://my/readme.md"))
			g.Expect(meta.Annotations["org.opencontainers.image.licenses"]).To(BeEquivalentTo("Apache-2.0"))

			po := &PushOptions{}
			for _, opt := range opts {
				opt(po)
			}
			switch po.layerType {
			case LayerTypeTarball:
				// Pull the artifact from registry and extract its contents to tmp
				tmpDir := t.TempDir()
				_, err := c.Pull(ctx, url, tmpDir)
				g.Expect(err).ToNot(HaveOccurred())
				// Walk the test directory and check that all files exist in the pulled artifact
				fsErr := filepath.Walk(tt.sourcePath, func(path string, info fs.FileInfo, err error) error {
					if !info.IsDir() {
						tmpPath := filepath.Join(tmpDir, strings.TrimPrefix(path, tt.sourcePath))
						if _, err := os.Stat(tmpPath); err != nil && os.IsNotExist(err) {
							return fmt.Errorf("path '%s' doesn't exist in archive", path)
						}
					}

					return nil
				})
				g.Expect(fsErr).ToNot(HaveOccurred())
			case LayerTypeStatic:
				// contents of uncompressed and compressed layer should be the same as file
				expectedBytes, err := os.ReadFile(tt.sourcePath)
				g.Expect(err).To(Not(HaveOccurred()))
				layers, err := image.Layers()
				g.Expect(err).ToNot(HaveOccurred())

				blob, err := layers[0].Uncompressed()
				g.Expect(err).ToNot(HaveOccurred())

				b, err := io.ReadAll(blob)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(b).To(BeEquivalentTo(expectedBytes))

				blob, err = layers[0].Compressed()
				g.Expect(err).ToNot(HaveOccurred())

				b, err = io.ReadAll(blob)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(b).To(BeEquivalentTo(expectedBytes))
			}
		})
	}
}

func Test_getLayerMediaType(t *testing.T) {
	tests := []struct {
		name              string
		ext               string
		expectedMediaType types.MediaType
	}{
		{
			name:              "default oci media type",
			expectedMediaType: oci.CanonicalMediaTypePrefix,
		},
		{
			name:              "oci media type with extension",
			ext:               "test",
			expectedMediaType: types.MediaType(fmt.Sprintf("%s.test", oci.CanonicalMediaTypePrefix)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got := getLayerMediaType(tt.ext)
			g.Expect(got).To(BeEquivalentTo(tt.expectedMediaType))
		})
	}
}
