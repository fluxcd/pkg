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
	"github.com/fluxcd/pkg/oci/auth/gcp"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
)

// Delete deletes a particular image from an OCI repository
// For GCR, GAR, the tag is deleted first, then the image
// as it doesn't let you delete an image if a tag still references it.
// This doesn't work with ECR/GHCR since they don't support DELETE according
// to the docker API spec.
func (c *Client) Delete(ctx context.Context, url string, tag string) error {
	ref, err := name.ParseReference(url)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	if tag != "" {
		img, err := crane.Pull(fmt.Sprintf("%s:%s", url, tag))
		if err != nil {
			return fmt.Errorf("error getting digest: %s", err)
		}

		hash, err := img.Digest()
		if err != nil {
			return err
		}

		// GCP registry doesn't permit deletion of image if it is still
		// referenced by a tag, so we will delete the tag first.
		if gcp.ValidHost(ref.Context().RegistryStr()) {
			err = crane.Delete(fmt.Sprintf("%s:%s", ref.Context(), tag))
			if err != nil {
				return err
			}
		}

		url = fmt.Sprintf("%s@%s", ref.Context(), hash.String())
	}

	return crane.Delete(url, c.optionsWithContext(ctx)...)
}
