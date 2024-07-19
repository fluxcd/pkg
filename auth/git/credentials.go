/*
Copyright 2024 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License.

You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package git

import (
	"context"
	"time"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AuthOptions contains options that can be used for authentication.
type AuthOptions struct {
	// ProviderOptions specifies the options to configure various authentication
	// providers.
	ProviderOptions ProviderOptions

	// Cache is a cache for storing auth configurations.
	Cache cache.Expirable[cache.StoreObject[Credentials]]
}

// ProviderOptions contains options to configure various authentication
// providers.
type ProviderOptions struct {
	AzureOpts []azure.ProviderOptFunc
}

// Credentials contains authentication data needed in order to access a Git
// repository.
type Credentials struct {
	BearerToken string `json:"bearerToken,omitempty"`
	ExpiresOn   time.Time
}

// ToSecretData returns the Credentials object in the format of the data found
// in Kubernetes Generic Secret.
func (c *Credentials) ToSecretData() map[string][]byte {
	var data map[string][]byte = make(map[string][]byte)

	if c.BearerToken != "" {
		data["bearerToken"] = []byte(c.BearerToken)
	}

	return data
}

func CacheCredentials(ctx context.Context, url string, authOpts *AuthOptions, creds *Credentials) {
	log := log.FromContext(ctx)
	if authOpts.Cache != nil {
		err := cacheObject(authOpts.Cache, *creds, url, creds.ExpiresOn)
		if err != nil {
			log.Error(err, "failed to cache credentials")
		}
	}
}

// GetCredentials returns authentication credentials for accessing the provided
// Git repository.
func GetCredentials(ctx context.Context, url string, provider string, authOpts *AuthOptions) (*Credentials, error) {
	var creds Credentials
	log := log.FromContext(ctx)

	if authOpts.Cache != nil {
		creds, exists, err := getObjectFromCache(authOpts.Cache, url)
		if err != nil {
			log.Error(err, "failed to get credential object from cache")
		}
		if exists {
			return &creds, nil
		}
	}

	switch provider {
	case auth.ProviderAzure:
		var opts []azure.ProviderOptFunc
		if authOpts != nil {
			opts = authOpts.ProviderOptions.AzureOpts
		}
		azureProvider := azure.NewProvider(opts...)
		accessToken, err := azureProvider.GetToken(ctx)
		if err != nil {
			return nil, err
		}
		creds = Credentials{
			BearerToken: accessToken.Token,
			ExpiresOn:   accessToken.ExpiresOn,
		}
	default:
		return nil, nil
	}

	return &creds, nil
}
