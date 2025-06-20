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

package authutils_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	authutils "github.com/fluxcd/pkg/auth/utils"
)

// TestGetArtifactRegistryCredentials_ProviderLookup tests the provider lookup
// behavior of GetArtifactRegistryCredentials. Full function testing is difficult
// because auth.GetToken is not interface-based and would require complex mocking.
func TestGetArtifactRegistryCredentials_ProviderLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		providerName           string
		expectUnsupportedError bool
	}{
		{
			name:                   "unsupported provider",
			providerName:           "unsupported-provider",
			expectUnsupportedError: true,
		},
		{
			name:                   "AWS provider",
			providerName:           "aws",
			expectUnsupportedError: false,
		},
		{
			name:                   "Azure provider",
			providerName:           "azure",
			expectUnsupportedError: false,
		},
		{
			name:                   "GCP provider",
			providerName:           "gcp",
			expectUnsupportedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			ctx := context.Background()

			authenticator, err := authutils.GetArtifactRegistryCredentials(ctx, tt.providerName, "registry.example.com")

			g.Expect(err).To(HaveOccurred())
			g.Expect(authenticator).To(BeNil())

			if tt.expectUnsupportedError {
				g.Expect(err).To(MatchError(authutils.ErrUnsupportedProvider))
			} else {
				g.Expect(err).NotTo(MatchError(authutils.ErrUnsupportedProvider))
			}
		})
	}
}
