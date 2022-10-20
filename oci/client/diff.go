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
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
)

// Diff compares the files included in an OCI image with the local files in the given path
// and returns an error if the contents is different
func (c *Client) Diff(ctx context.Context, url, dir string, ignorePaths []string) error {
	_, err := name.ParseReference(url)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	tmpBuildDir, err := os.MkdirTemp("", "ocibuild")
	if err != nil {
		return fmt.Errorf("creating temp build dir failed: %w", err)
	}
	defer os.RemoveAll(tmpBuildDir)

	tmpFile := filepath.Join(tmpBuildDir, "artifact.tgz")

	if err := c.Build(tmpFile, dir, ignorePaths); err != nil {
		return fmt.Errorf("building artifact failed: %w", err)
	}

	f, err := os.Open(tmpFile)
	if err != nil {
		return fmt.Errorf("opening artifact failed: %w", err)
	}
	defer f.Close()

	h1 := sha256.New()
	if _, err := io.Copy(h1, f); err != nil {
		return fmt.Errorf("calculating artifact hash failed: %w", err)
	}

	img, err := crane.Pull(url, c.optionsWithContext(ctx)...)
	if err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to list layers: %w", err)
	}

	if len(layers) < 1 {
		return fmt.Errorf("no layers found in artifact")
	}

	blob, err := layers[0].Compressed()
	if err != nil {
		return fmt.Errorf("extracting first layer failed: %w", err)
	}

	h2 := sha256.New()
	if _, err := io.Copy(h2, blob); err != nil {
		return fmt.Errorf("calculating hash failed: %w", err)
	}

	if bytes.Compare(h1.Sum(nil), h2.Sum(nil)) != 0 {
		return fmt.Errorf("the remote artifact contents differs from the local one")
	}

	return nil
}
