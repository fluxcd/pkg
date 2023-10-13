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

package registry

import (
	"context"
	"time"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	"github.com/google/go-containerregistry/pkg/authn"
)

// GetAuthenticator returns an authenticator that can provide credentials to
// access the provided registry.
// If caching is enabled and authOpts.CacheKey is not blank, the authentication
// config is cached according to the ttl advertised by the registry provider.
func GetAuthenticator(ctx context.Context, registry string, provider string, authOpts *auth.AuthOptions) (authn.Authenticator, error) {
	var authConfig authn.AuthConfig

	cache := auth.GetCache()
	if cache != nil && authOpts != nil && authOpts.CacheKey != "" {
		val, found := cache.Get(authOpts.CacheKey)
		if found {
			authConfig = val.(authn.AuthConfig)
			return authn.FromConfig(authConfig), nil
		}
	}

	var err error
	var expiresIn time.Duration
	switch provider {
	case auth.ProviderAWS:
		var opts []aws.ProviderOptFunc
		if authOpts != nil {
			opts = authOpts.ProviderOptions.AwsOpts
		}
		awsProvider := aws.NewProvider(opts...)
		authConfig, expiresIn, err = awsProvider.GetECRAuthConfig(ctx, registry)
	case auth.ProviderAzure:
		var opts []azure.ProviderOptFunc
		scopeOpt := azure.GetScopeProiderOption(registry)
		if scopeOpt != nil {
			opts = append(opts, scopeOpt)
		}
		if authOpts != nil {
			opts = authOpts.ProviderOptions.AzureOpts
		}

		azureProvider := azure.NewProvider(opts...)
		authConfig, expiresIn, err = azureProvider.GetACRAuthConfig(ctx, registry)
	case auth.ProviderGCP:
		var opts []gcp.ProviderOptFunc
		if authOpts != nil {
			opts = authOpts.ProviderOptions.GcpOpts
		}
		gcpProvider := gcp.NewProvider(opts...)
		authConfig, expiresIn, err = gcpProvider.GetGARAuthConfig(ctx)
	default:
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if cache != nil && authOpts != nil && authOpts.CacheKey != "" {
		if err := cache.Set(authOpts.CacheKey, authConfig, expiresIn); err != nil {
			return nil, err
		}
	}
	return authn.FromConfig(authConfig), nil
}
