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

package authutils

import (
	"context"

	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/fluxcd/pkg/auth"
)

// GetArtifactRegistryCredentials retrieves the credentials for the specified
// artifact repository using the specified provider. It returns an
// authn.Authenticator that can be used to authenticate with the registry.
func GetArtifactRegistryCredentials(ctx context.Context,
	providerName, artifactRepository string,
	opts ...auth.Option) (authn.Authenticator, error) {

	provider := ProviderByName(providerName)
	if provider == nil {
		return nil, ErrUnsupportedProvider
	}

	opts = append(opts, auth.WithArtifactRepository(artifactRepository))

	token, err := auth.GetToken(ctx, provider, opts...)
	if err != nil {
		return nil, err
	}

	authenticator, ok := token.(authn.Authenticator)
	if !ok {
		return nil, ErrProviderDoesNotSupportRegistry
	}

	return authenticator, nil
}
