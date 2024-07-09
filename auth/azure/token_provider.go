/*
Copyright 2024 The Flux authors Licensed under the Apache License, Version 2.0
(the "License"); you may not use this file except in compliance with the
License.

You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	AzureDevOpsRestApiScope = "499b84ac-1321-427f-aa17-267ca6975798/.default"
)

// Provider is an authentication provider for Azure.
type Provider struct {
	credential azcore.TokenCredential
	scopes     []string
}

// ProviderOptFunc enables specifying options for the provider.
type ProviderOptFunc func(*Provider)

// NewProvider returns a new authentication provider for Azure.
func NewProvider(opts ...ProviderOptFunc) *Provider {
	p := &Provider{}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// WithCredential configures the credential to use to fetch the resource manager
// token.
func WithCredential(cred azcore.TokenCredential) ProviderOptFunc {
	return func(p *Provider) {
		p.credential = cred
	}
}

func WithAzureDevOpsScope() ProviderOptFunc {
	return func(p *Provider) {
		p.scopes = []string{AzureDevOpsRestApiScope}
	}
}

func (p *Provider) GetToken(ctx context.Context) (*azcore.AccessToken, error) {
	if len(p.scopes) == 0 {
		return nil, fmt.Errorf("error scopes must be specified")
	}

	if p.credential == nil {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, err
		}
		p.credential = cred
	}

	accessToken, err := p.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: p.scopes,
	})
	if err != nil {
		return nil, err
	}

	return &accessToken, nil
}
