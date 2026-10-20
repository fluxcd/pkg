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

package gcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/google/go-containerregistry/pkg/authn"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google/externalaccount"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/option"
	htransport "google.golang.org/api/transport/http"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	auth "github.com/fluxcd/pkg/auth"
)

// ProviderName is the name of the GCP authentication provider.
const ProviderName = "gcp"

var scopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	"https://www.googleapis.com/auth/userinfo.email",
}

// Provider implements the auth.Provider interface for GCP authentication.
type Provider struct{ Implementation }

// GetName implements auth.Provider.
func (Provider) GetName() string {
	return ProviderName
}

// NewControllerToken implements auth.Provider.
func (p Provider) NewControllerToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {
	var o auth.Options
	o.Apply(opts...)

	ctx = context.WithValue(ctx, oauth2.HTTPClient, o.GetHTTPClient())

	src, err := p.impl().DefaultTokenSource(ctx, scopes...)
	if err != nil {
		return nil, err
	}
	token, err := src.Token()
	if err != nil {
		return nil, err
	}

	return &Token{*token}, nil
}

// GetAudiences implements auth.Provider.
func (p Provider) GetAudiences(ctx context.Context, opts ...auth.Option) ([]string, error) {

	var o auth.Options
	o.Apply(opts...)

	// GCP Workload Identity Federation is only possible without a configured
	// provider if we are in GKE, which has a built-in one that only accepts
	// a single specific audience. This audience can be computed from information
	// provided by the GKE metadata server.
	if o.GCPWorkloadIdentityProvider == "" {
		audience, err := gkeMetadata.getTokenAudience(ctx)
		if err != nil {
			return nil, err
		}
		return []string{audience}, nil
	}

	// With a configured workload identity provider, custom credential audiences
	// can be used.
	if len(o.CredentialAudiences) > 0 {
		return o.CredentialAudiences, nil
	}
	return []string{cryptov1.GetDefaultAudienceGCP(o.GCPWorkloadIdentityProvider)}, nil
}

// GetIdentity implements auth.Provider.
func (Provider) GetIdentity(opts ...auth.Option) (string, error) {
	var o auth.Options
	o.Apply(opts...)
	return o.GCPServiceAccount, nil
}

// NewTokenForServiceAccount implements auth.Provider.
func (p Provider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	// The STS exchange audience identifies the workload identity provider. It is
	// intentionally not derived from the credential audiences option: the latter
	// is the audience for the OIDC token, which the provider validates against
	// its configured allowed audiences. The fundamental reason for this difference
	// is that the STS exchange audience is used to look up the workload identity
	// provider resource in the STS service, while the credential audiences are
	// user-defined configuration stored *inside* that workload identity provider
	// resource. After using the exchange audience to locate the workload identity
	// provider resource, STS then validates the OIDC token audiences against the
	// ones configured inside the resource.
	var exchangeAudience string
	if o.GCPWorkloadIdentityProvider != "" {
		// The exchange audience is the workload identity provider resource name
		// in its `//` form as defined in https://google.aip.dev/auth/4117.
		exchangeAudience = fmt.Sprintf("//iam.googleapis.com/%s", o.GCPWorkloadIdentityProvider)
	} else {
		// Workload identity federation is only possible without a configured provider
		// if we are in GKE, which has a built-in provider. In this case, the exchange
		// audience can be computed from information provided by the GKE metadata server.
		var err error
		exchangeAudience, err = gkeMetadata.getExchangeAudience(ctx)
		if err != nil {
			return nil, err
		}
	}

	conf := externalaccount.Config{
		UniverseDomain:       "googleapis.com",
		Audience:             exchangeAudience,
		SubjectTokenType:     "urn:ietf:params:oauth:token-type:jwt",
		TokenURL:             "https://sts.googleapis.com/v1/token",
		SubjectTokenSupplier: StaticTokenSupplier(oidcToken),
		Scopes:               scopes,
	}

	email := o.GCPServiceAccount

	if email != "" { // impersonation
		conf.ServiceAccountImpersonationURL = fmt.Sprintf(
			"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken",
			email)
	} else { // direct access
		conf.TokenInfoURL = "https://sts.googleapis.com/v1/introspect"
	}

	ctx = context.WithValue(ctx, oauth2.HTTPClient, o.GetHTTPClient())

	src, err := p.impl().NewTokenSource(ctx, conf)
	if err != nil {
		return nil, err
	}
	token, err := src.Token()
	if err != nil {
		return nil, err
	}

	return &Token{*token}, nil
}

// GetAccessTokenOptionsForArtifactRepository implements auth.Provider.
func (Provider) GetAccessTokenOptionsForArtifactRepository(string, ...auth.Option) ([]auth.Option, error) {
	// GCP does not require any special options to retrieve access tokens.
	return nil, nil
}

// ParseArtifactRepository implements auth.Provider.
func (Provider) ParseArtifactRepository(artifactRepository string) (string, error) {
	return cryptov1.ParseArtifactRepository(ProviderName, artifactRepository)
}

// NewArtifactRegistryCredentials implements auth.Provider.
func (Provider) NewArtifactRegistryCredentials(_ context.Context, _ string,
	accessToken auth.Token, _ ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {

	t := accessToken.(*Token)

	return &auth.ArtifactRegistryCredentials{
		Authenticator: authn.FromConfig(authn.AuthConfig{
			Username: "oauth2accesstoken",
			Password: t.AccessToken,
		}),
		ExpiresAt: t.Expiry,
	}, nil
}

// GetAccessTokenOptionsForCluster implements auth.Provider.
func (Provider) GetAccessTokenOptionsForCluster(opts ...auth.Option) ([][]auth.Option, error) {
	// A single token is needed. No options.
	return [][]auth.Option{{}}, nil
}

// NewRESTConfig implements auth.Provider.
func (p Provider) NewRESTConfig(ctx context.Context, accessTokens []auth.Token,
	opts ...auth.Option) (*auth.RESTConfig, error) {

	token := accessTokens[0].(*Token)

	var o auth.Options
	o.Apply(opts...)

	// Describe the cluster resource to get missing CA or endpoint.
	host := o.ClusterAddress
	caData := []byte(o.CAData)
	if host == "" || len(caData) == 0 {
		cluster := o.ClusterResource

		// Create client for describing the cluster resource.
		baseTransport := http.DefaultTransport.(*http.Transport).Clone()
		if p := o.ProxyURL; p != nil {
			baseTransport.Proxy = http.ProxyURL(p)
		}
		transport, err := htransport.NewTransport(ctx, baseTransport, option.WithTokenSource(token.source()))
		if err != nil {
			return nil, fmt.Errorf("failed to create google http transport for describing GKE cluster: %w", err)
		}
		client, err := container.NewService(ctx, option.WithHTTPClient(&http.Client{Transport: transport}))
		if err != nil {
			return nil, fmt.Errorf("failed to create client for describing GKE cluster: %w", err)
		}

		// Describe the cluster resource.
		clusterResource, err := p.impl().GetCluster(ctx, cluster, client)
		if err != nil {
			return nil, fmt.Errorf("failed to describe GKE cluster '%s': %w", cluster, err)
		}

		// Update host and CA with cluster details.
		if host == "" {
			host = clusterResource.Endpoint
		}
		if len(caData) == 0 {
			caData, err = base64.StdEncoding.DecodeString(clusterResource.MasterAuth.ClusterCaCertificate)
			if err != nil {
				return nil, fmt.Errorf("failed to decode GKE CA certificate: %w", err)
			}
		}
	}

	// Build and return the REST config.
	return &auth.RESTConfig{
		Host:        host,
		BearerToken: token.AccessToken,
		CAData:      caData,
		ExpiresAt:   token.Expiry,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}
