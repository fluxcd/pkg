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

package utils

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/fluxcd/pkg/auth"
)

// GetArtifactRegistryCredentials retrieves the registry credentials for the
// specified artifact repository and provider.
func GetArtifactRegistryCredentials(ctx context.Context, providerName string,
	artifactRepository string, opts ...auth.Option) (authn.Authenticator, error) {

	provider, err := ProviderByName[auth.ArtifactRegistryCredentialsProvider](providerName)
	if err != nil {
		return nil, err
	}

	return auth.GetArtifactRegistryCredentials(ctx, provider, artifactRepository, opts...)
}
