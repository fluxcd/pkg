/*
Copyright 2023 The Flux authors

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
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	"github.com/fluxcd/pkg/auth/github"
	corev1 "k8s.io/api/core/v1"
)

const (
	ProviderAWS    = "aws"
	ProviderAzure  = "azure"
	ProviderGCP    = "gcp"
	ProviderGitHub = "github"
)

// AuthOptions contains options that can be used for authentication.
type AuthOptions struct {
	// Secret contains information that can be used to obtain the required
	// set of credentials.
	Secret *corev1.Secret

	// ProviderOptions specifies the options to configure various authentication
	// providers.
	ProviderOptions ProviderOptions

	// CacheOptions specifies the options to configure caching behavior of the
	// authentication credentials.
	CacheOptions CacheOptions
}

// GetCache returns the cache to use for fetching/storing authentication
// credentials.
func (a *AuthOptions) GetCache() Store {
	if a.CacheOptions.Cache != nil {
		return a.CacheOptions.Cache
	}
	return GetCache()
}

// CacheOptions contains options to configure the caching behavior of the
// authentication credentials.
type CacheOptions struct {
	// Key is the key to use for caching the authentication credentials.
	Key string

	// Cache is the Store to use for caching the authentication credentials.
	// If specified, then the global cache specified through `auth.InitCache()`
	// is ignored and the credentials are cached in this Store instead.
	Cache Store
}

// ProviderOptions contains options to configure various authentication
// providers.
type ProviderOptions struct {
	AwsOpts    []aws.ProviderOptFunc
	AzureOpts  []azure.ProviderOptFunc
	GcpOpts    []gcp.ProviderOptFunc
	GitHubOpts []github.ProviderOptFunc
}
