/*
Copyright 2024 The Flux authors

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
	"fmt"
	"time"

	"github.com/fluxcd/pkg/auth/azure"
)

const (
	ProviderAzure = "azure"
)

// Credentials contains authentication data needed in order to access a Git
// repository.
type Credentials struct {
	BearerToken string
}

// GetCredentials returns authentication credentials for accessing the provided
// Git repository.
func GetCredentials(ctx context.Context, providerOpts *ProviderOptions) (*Credentials, time.Time, error) {
	var (
		creds     Credentials
		expiresOn time.Time
	)

	if providerOpts == nil {
		return nil, expiresOn, fmt.Errorf("provider options are not specified")
	}

	switch providerOpts.Name {
	case ProviderAzure:
		opts := providerOpts.AzureOpts
		if providerOpts.AzureOpts == nil {
			opts = []azure.OptFunc{
				azure.WithAzureDevOpsScope(),
			}
		}
		client, err := azure.New(opts...)
		if err != nil {
			return nil, expiresOn, err
		}
		accessToken, err := client.GetToken(ctx)
		if err != nil {
			return nil, expiresOn, err
		}

		creds = Credentials{
			BearerToken: accessToken.Token,
		}
		return &creds, accessToken.ExpiresOn, nil
	default:
		return nil, expiresOn, fmt.Errorf("invalid provider")
	}
}
