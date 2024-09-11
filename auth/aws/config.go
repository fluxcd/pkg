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

package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

// Provider is an authentication provider for AWS.
type Provider struct {
	optFns []func(*config.LoadOptions) error
	config *aws.Config
}

// ProviderOptFunc enables specifying options for the provider.
type ProviderOptFunc func(*Provider)

// NewProvider returns a new authentication provider for AWS.
func NewProvider(opts ...ProviderOptFunc) *Provider {
	p := &Provider{}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// WithRegion configures the AWS region.
func WithRegion(region string) ProviderOptFunc {
	return func(p *Provider) {
		p.optFns = append(p.optFns, config.WithRegion(region))
	}
}

// WithOptFns configures the AWS config with the provided load options.
func WithOptFns(optFns []func(*config.LoadOptions) error) ProviderOptFunc {
	return func(p *Provider) {
		p.optFns = append(p.optFns, optFns...)
	}
}

// WithConfig specifies the custom AWS config to use.
func WithConfig(config aws.Config) ProviderOptFunc {
	return func(p *Provider) {
		p.config = &config
	}
}

// GetConfig returns the default config constructed using any options that the
// provider was configured with. If OIDC/IRSA has been configured for the EKS
// cluster, then the config object will also be configured with the necessary
// credentials. The returned config object can be used to fetch tokens to access
// particular AWS services.
func (p *Provider) GetConfig(ctx context.Context) (aws.Config, error) {
	if p.config != nil {
		return *p.config, nil
	}
	cfg, err := config.LoadDefaultConfig(ctx, p.optFns...)
	return cfg, err
}
