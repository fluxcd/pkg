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

package git

import (
	"context"
	"time"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	"github.com/fluxcd/pkg/auth/github"
)

const GitHubAccessTokenUsername = "x-access-token"

// Credentials contains the various authentication data needed
// in order to access a Git repository.
type Credentials struct {
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	BearerToken string `json:"bearerToken,omitempty"`
}

// ToSecretData returns the Credentials object in the format
// of the data found in Kubernetes Generic Secret.
func (c *Credentials) ToSecretData() map[string][]byte {
	var data map[string][]byte

	if c.BearerToken != "" {
		data["bearerToken"] = []byte(c.BearerToken)
	}
	if c.Password != "" {
		data["password"] = []byte(c.Password)
	}
	if c.Username != "" {
		data["username"] = []byte(c.Username)
	}
	return data
}

// GetCredentials returns authentication credentials for accessing the provided
// Git repository.
// The authentication credentials will be cached if `authOpts.CacheOptions.Key`
// is not blank and caching is enabled. Caching can be enabled by either calling
// `auth.InitCache()`  or specifying a cache via `authOpts.CacheOptions.Cache`.
// The credentials are cached according to the ttl advertised by the registry
// provider.
func GetCredentials(ctx context.Context, provider string, authOpts *auth.AuthOptions) (*Credentials, error) {
	var creds Credentials

	var cache auth.Store
	if authOpts != nil {
		cache = authOpts.GetCache()
		if cache != nil && authOpts.CacheOptions.Key != "" {
			val, found := cache.Get(authOpts.CacheOptions.Key)
			if found {
				creds = val.(Credentials)
				return &creds, nil
			}
		}
	}

	var expiresIn time.Duration
	switch provider {
	case auth.ProviderAzure:
		var opts []azure.ProviderOptFunc
		if authOpts != nil {
			opts = authOpts.ProviderOptions.AzureOpts
		}
		azureProvider := azure.NewProvider(opts...)

		armToken, err := azureProvider.GetResourceManagerToken(ctx)
		if err != nil {
			return nil, err
		}
		creds = Credentials{
			BearerToken: armToken.Token,
		}
		expiresIn = armToken.ExpiresOn.UTC().Sub(time.Now().UTC())
	case auth.ProviderGCP:
		var opts []gcp.ProviderOptFunc
		if authOpts != nil {
			opts = authOpts.ProviderOptions.GcpOpts
		}
		gcpProvider := gcp.NewProvider(opts...)

		saToken, err := gcpProvider.GetServiceAccountToken(ctx)
		if err != nil {
			return nil, err
		}
		email, err := gcpProvider.GetServiceAccountEmail(ctx)
		if err != nil {
			return nil, err
		}

		creds = Credentials{
			Username: email,
			Password: saToken.AccessToken,
		}
		expiresIn = time.Duration(saToken.ExpiresIn)
	case auth.ProviderGitHub:
		var opts []github.ProviderOptFunc
		if authOpts != nil {
			if authOpts.Secret != nil {
				opts = append(opts, github.WithSecret(*authOpts.Secret))
			}
			opts = append(opts, authOpts.ProviderOptions.GitHubOpts...)
		}

		ghProvider, err := github.NewProvider(opts...)
		if err != nil {
			return nil, err
		}

		appToken, err := ghProvider.GetAppToken(ctx)
		if err != nil {
			return nil, err
		}
		creds = Credentials{
			Username: GitHubAccessTokenUsername,
			Password: appToken.Token,
		}
		expiresIn = appToken.ExpiresIn
	default:
		return nil, nil
	}

	if cache != nil && authOpts != nil && authOpts.CacheOptions.Key != "" {
		if err := cache.Set(authOpts.CacheOptions.Key, creds, expiresIn); err != nil {
			return nil, err
		}
	}
	return &creds, nil
}
