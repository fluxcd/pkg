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

package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
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

// WithCredential configures the credential to use to fetch the resource
// manager token.
func WithCredential(cred azcore.TokenCredential) ProviderOptFunc {
	return func(p *Provider) {
		p.credential = cred
	}
}

// WithAzureGovtScope configures the scopes of all Azure calls to
// target Azure Government.
func WithAzureGovtScope() ProviderOptFunc {
	return func(p *Provider) {
		p.scopes = []string{cloud.AzureGovernment.Services[cloud.ResourceManager].Endpoint + "/" + ".default"}
	}
}

// WithAzureChinaScope configures the scopes of all Azure calls to
// target Azure China.
func WithAzureChinaScope() ProviderOptFunc {
	return func(p *Provider) {
		p.scopes = []string{cloud.AzureChina.Services[cloud.ResourceManager].Endpoint + "/" + ".default"}
	}
}

// GetResourceManagerToken fetches the Azure Resource Manager token using the
// credential that the provider is configured with. If it isn't, then a new
// credential chain is constructed using the default method, which includes
// trying to use Workload Identity, Managed Identity, etc.
// By default, the scope of the request targets the Azure Public cloud, but this
// is configurable using WithAzureGovtScope or WithAzureChinaScope.
func (p *Provider) GetResourceManagerToken(ctx context.Context) (*azcore.AccessToken, error) {
	if p.credential == nil {
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, err
		}
		p.credential = cred
	}
	if len(p.scopes) == 0 {
		p.scopes = []string{cloud.AzurePublic.Services[cloud.ResourceManager].Endpoint + "/" + ".default"}
	}

	accessToken, err := p.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: p.scopes,
	})
	if err != nil {
		return nil, err
	}

	return &accessToken, nil
}
