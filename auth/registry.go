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

package auth

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/fluxcd/pkg/cache"
)

// ArtifactRegistryCredentialsProvider is an interface that defines methods
// for retrieving credentials for artifact registries from cloud providers.
type ArtifactRegistryCredentialsProvider interface {
	Provider

	// GetAccessTokenOptionsForArtifactRepository returns the options that must be
	// passed to the provider to retrieve access tokens for an artifact repository.
	GetAccessTokenOptionsForArtifactRepository(artifactRepository string) ([]Option, error)

	// ParseArtifactRepository parses the artifact repository to verify
	// it's a valid repository for the provider. As a result, it returns
	// the input required for the provider to issue registry credentials.
	// This input is included in the cache key for the issued credentials.
	ParseArtifactRepository(artifactRepository string) (string, error)

	// NewArtifactRegistryCredentials takes the registry input extracted by
	// ParseArtifactRepository() and an access token and returns credentials
	// that can be used to authenticate with the registry.
	NewArtifactRegistryCredentials(ctx context.Context, registryInput string,
		accessToken Token, opts ...Option) (*ArtifactRegistryCredentials, error)
}

// ArtifactRegistryCredentials is a particular type implementing the Token interface
// for credentials that can be used to authenticate against an artifact registry
// from a cloud provider.
type ArtifactRegistryCredentials struct {
	authn.Authenticator
	ExpiresAt time.Time
}

// GetDuration implements Token.
func (a *ArtifactRegistryCredentials) GetDuration() time.Duration {
	return time.Until(a.ExpiresAt)
}

// GetRegistryFromArtifactRepository returns the registry from the artifact repository.
func GetRegistryFromArtifactRepository(artifactRepository string) (string, error) {
	registry := strings.TrimSuffix(artifactRepository, "/")
	if strings.ContainsRune(registry, '/') {
		ref, err := name.ParseReference(registry)
		if err != nil {
			return "", fmt.Errorf("failed to parse artifact repository '%s': %w",
				artifactRepository, err)
		}
		return ref.Context().RegistryStr(), nil
	}
	return registry, nil
}

// GetArtifactRegistryCredentials retrieves the registry credentials for the
// specified artifact repository and provider.
func GetArtifactRegistryCredentials(ctx context.Context, provider ArtifactRegistryCredentialsProvider,
	artifactRepository string, opts ...Option) (*ArtifactRegistryCredentials, error) {

	registryInput, err := provider.ParseArtifactRepository(artifactRepository)
	if err != nil {
		return nil, err
	}

	// First, we need an access token. This cannot be retrieved inside the
	// cache lock, otherwise we reach a deadlock.
	accessTokenOpts, err := provider.GetAccessTokenOptionsForArtifactRepository(artifactRepository)
	if err != nil {
		return nil, err
	}
	accessTokenOpts = append(slices.Clone(opts), accessTokenOpts...)
	accessToken, err := GetAccessToken(ctx, provider, accessTokenOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token for artifact registry: %w", err)
	}

	// Prepare a function to create new credentials.
	newArtifactRegistryCredentials := func() (*ArtifactRegistryCredentials, error) {
		creds, err := provider.NewArtifactRegistryCredentials(ctx, registryInput, accessToken, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create artifact registry credentials: %w", err)
		}
		return creds, nil
	}

	var o Options
	o.Apply(opts...)

	// Bail out early if cache is disabled.
	if o.Cache == nil {
		return newArtifactRegistryCredentials()
	}

	// Build cache key.
	var saInfo *serviceAccountInfo
	if o.ShouldGetServiceAccount() {
		var err error
		saInfo, err = getServiceAccountInfo(ctx, provider, o.Client, opts...)
		if err != nil {
			return nil, err
		}
	}
	accessTokenCacheKey := buildAccessTokenCacheKey(provider, saInfo, accessTokenOpts...)
	cacheKey := buildCacheKey(
		fmt.Sprintf("accessTokenCacheKey=%s", accessTokenCacheKey),
		fmt.Sprintf("artifactRepositoryCacheKey=%s", registryInput))

	// Build involved object details.
	kind := o.InvolvedObject.Kind
	name := o.InvolvedObject.Name
	namespace := o.InvolvedObject.Namespace
	operation := o.InvolvedObject.Operation

	// Get credentials from cache.
	creds, _, err := o.Cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (cache.Token, error) {
		return newArtifactRegistryCredentials()
	}, cache.WithInvolvedObject(kind, name, namespace, operation))
	if err != nil {
		return nil, err
	}

	return creds.(*ArtifactRegistryCredentials), nil
}
