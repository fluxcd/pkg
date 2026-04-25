/*
Copyright 2026 The Flux authors

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
	"errors"
	"fmt"
	"net/url"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/cache"
)

// GitCredentialsProvider is an interface that defines methods for
// retrieving credentials for Git repositories from cloud providers.
type GitCredentialsProvider interface {
	Provider

	// GetAccessTokenOptionsForGitRepository returns the options that must be
	// passed to the provider to retrieve access tokens for the given Git
	// repository URL.
	GetAccessTokenOptionsForGitRepository(gitURL *url.URL) ([]Option, error)

	// ParseGitRepository parses the given Git repository URL to verify
	// it's a valid repository URL for the provider. As a result it returns
	// an opaque per-repository input required for issuing Git credentials.
	// This input is included in the cache key for the issued credentials,
	// which is critical for providers whose credentials depend on the URL
	// (e.g. AWS CodeCommit, whose credentials are a SigV4 signature over
	// the request URL).
	ParseGitRepository(gitURL *url.URL) (string, error)

	// NewGitCredentials takes the input extracted by ParseGitRepository()
	// and an access token and returns credentials that can be used to
	// authenticate against the Git repository.
	NewGitCredentials(ctx context.Context, gitInput string,
		accessToken Token, opts ...Option) (*GitCredentials, error)
}

// GitCredentials contains authentication data needed in order to access a Git
// repository.
type GitCredentials struct {
	BearerToken string
	Username    string
	Password    string
	ExpiresAt   time.Time
}

// GetDuration implements Token.
func (g *GitCredentials) GetDuration() time.Duration {
	return time.Until(g.ExpiresAt)
}

// GetGitCredentials retrieves the Git credentials for the Git repository URL
// set via WithGitURL and the specified provider.
func GetGitCredentials(ctx context.Context, provider GitCredentialsProvider,
	opts ...Option) (*GitCredentials, error) {

	var o Options
	o.Apply(opts...)

	gitURL := o.GitURL
	if gitURL == nil {
		return nil, errors.New("a Git repository URL is required for issuing Git credentials")
	}

	gitInput, err := provider.ParseGitRepository(gitURL)
	if err != nil {
		return nil, err
	}

	// First, we need an access token. This cannot be retrieved inside the
	// cache lock, otherwise we reach a deadlock.
	accessTokenOpts, err := provider.GetAccessTokenOptionsForGitRepository(gitURL)
	if err != nil {
		return nil, err
	}
	accessTokenOpts = append(slices.Clone(opts), accessTokenOpts...)
	accessToken, err := GetAccessToken(ctx, provider, accessTokenOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token for Git repository: %w", err)
	}

	// Prepare a function to create new credentials.
	newGitCredentials := func() (*GitCredentials, error) {
		creds, err := provider.NewGitCredentials(ctx, gitInput, accessToken, opts...)
		if err != nil {
			return nil, fmt.Errorf("failed to create Git credentials: %w", err)
		}
		return creds, nil
	}

	// Bail out early if cache is disabled.
	if o.Cache == nil {
		return newGitCredentials()
	}

	// Build cache key.
	var serviceAccount *corev1.ServiceAccount
	var providerIdentity string
	var audiences []string
	if o.ShouldGetServiceAccountToken() {
		var err error
		saRef := client.ObjectKey{
			Name:      o.ServiceAccountName,
			Namespace: o.ServiceAccountNamespace,
		}
		serviceAccount, audiences, providerIdentity, err =
			getServiceAccountAndProviderInfo(ctx, provider, o.Client, saRef, opts...)
		if err != nil {
			return nil, err
		}
	}
	accessTokenCacheKey := buildAccessTokenCacheKey(provider, audiences,
		providerIdentity, serviceAccount, accessTokenOpts...)
	cacheKey := buildCacheKey(
		fmt.Sprintf("accessTokenCacheKey=%s", accessTokenCacheKey),
		fmt.Sprintf("gitRepositoryCacheKey=%s", gitInput))

	// Build involved object details.
	kind := o.InvolvedObject.Kind
	name := o.InvolvedObject.Name
	namespace := o.InvolvedObject.Namespace
	operation := o.InvolvedObject.Operation

	// Get credentials from cache.
	creds, _, err := o.Cache.GetOrSet(ctx, cacheKey, func(ctx context.Context) (cache.Token, error) {
		return newGitCredentials()
	}, cache.WithInvolvedObject(kind, name, namespace, operation))
	if err != nil {
		return nil, err
	}

	return creds.(*GitCredentials), nil
}
