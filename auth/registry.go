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
	"time"

	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/fluxcd/pkg/auth/kubernetes"
	"github.com/fluxcd/pkg/cache"
)

// ArtifactRegistryCredentialsProvider is an interface that defines methods
// for retrieving credentials for artifact registries from cloud providers.
type ArtifactRegistryCredentialsProvider interface {
	Provider

	// GetAccessCredentialOptionsForArtifactRepository returns the options that must be
	// passed to the provider to retrieve access credentials for an artifact repository.
	GetAccessCredentialOptionsForArtifactRepository(artifactRepository string,
		opts ...Option) ([]Option, error)

	// ParseArtifactRepository parses the artifact repository to verify
	// it's a valid repository for the provider. As a result, it returns
	// the input required for the provider to issue registry credentials.
	// This input is included in the cache key for the issued credentials.
	ParseArtifactRepository(artifactRepository string) (string, error)

	// NewArtifactRegistryCredentials takes the registry input extracted by
	// ParseArtifactRepository() and an access credential and returns
	// credentials that can be used to authenticate with the registry.
	NewArtifactRegistryCredentials(ctx context.Context, registryInput string,
		accessCredential Credential, opts ...Option) (*ArtifactRegistryCredentials, error)
}

// ArtifactRegistryCredentials is a particular type implementing the Credential
// interface for credentials that can be used to authenticate against an
// artifact registry from a cloud provider.
type ArtifactRegistryCredentials struct {
	authn.Authenticator
	ExpiresAt time.Time
}

// GetDuration implements Credential.
func (a *ArtifactRegistryCredentials) GetDuration() time.Duration {
	return time.Until(a.ExpiresAt)
}

// GetArtifactRegistryCredentials retrieves the registry credentials for the
// specified artifact repository and provider.
func GetArtifactRegistryCredentials(ctx context.Context, provider ArtifactRegistryCredentialsProvider,
	artifactRepository string, opts ...Option) (*ArtifactRegistryCredentials, error) {

	registryInput, err := provider.ParseArtifactRepository(artifactRepository)
	if err != nil {
		return nil, err
	}

	// First, we need an access credential. This cannot be retrieved inside
	// the cache lock, otherwise we reach a deadlock.
	accessCredentialOpts, err := provider.GetAccessCredentialOptionsForArtifactRepository(artifactRepository, opts...)
	if err != nil {
		return nil, err
	}
	accessCredentialOpts = append(slices.Clone(opts), accessCredentialOpts...)
	accessCredential, err := NewAccessCredential(ctx, provider, accessCredentialOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get access credential for artifact registry: %w", err)
	}

	// Prepare a function to create new credentials.
	newArtifactRegistryCredentials := func() (*ArtifactRegistryCredentials, error) {
		creds, err := provider.NewArtifactRegistryCredentials(ctx, registryInput, accessCredential, opts...)
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
	var serviceAccount *kubernetes.ServiceAccountReference
	var providerIdentity string
	var audiences []string
	if o.objectLevelWorkloadIdentity() {
		var err error
		serviceAccount, audiences, providerIdentity, err =
			getServiceAccountAndProviderInfo(ctx, provider, opts...)
		if err != nil {
			return nil, err
		}
	}
	accessCredentialCacheKey := buildAccessCredentialCacheKey(provider, audiences,
		providerIdentity, serviceAccount, accessCredentialOpts...)
	cacheKey := buildCacheKey(
		fmt.Sprintf("accessCredentialCacheKey=%s", accessCredentialCacheKey),
		fmt.Sprintf("artifactRepositoryCacheKey=%s", registryInput))

	// Get credentials from cache.
	creds, _, err := o.Cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (cache.Token, error) {
		return newArtifactRegistryCredentials()
	}, cache.WithInvolvedObject(o.involvedObject()))
	if err != nil {
		return nil, err
	}

	return creds.(*ArtifactRegistryCredentials), nil
}
