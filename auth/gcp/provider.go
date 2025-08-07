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
	"regexp"

	"github.com/google/go-containerregistry/pkg/authn"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google/externalaccount"
	"google.golang.org/api/container/v1"
	"google.golang.org/api/option"
	htransport "google.golang.org/api/transport/http"
	corev1 "k8s.io/api/core/v1"

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
func (Provider) GetAudiences(ctx context.Context, serviceAccount corev1.ServiceAccount) ([]string, error) {

	// Check if a workload identity provider is specified in the service account.
	// If so, the current cluster is not GKE and the audience is the provider itself.
	audience, err := getWorkloadIdentityProviderAudience(serviceAccount)
	if err != nil {
		return nil, err
	}
	if audience != "" {
		return []string{audience}, nil
	}

	// Assume we are in GKE. In this case, the audience is the workload identity pool.
	audience, err = gkeMetadata.workloadIdentityPool(ctx)
	if err != nil {
		return nil, err
	}
	return []string{audience}, nil
}

// GetIdentity implements auth.Provider.
func (Provider) GetIdentity(serviceAccount corev1.ServiceAccount) (string, error) {
	email, err := getServiceAccountEmail(serviceAccount)
	if err != nil {
		return "", err
	}
	return email, nil
}

// NewTokenForServiceAccount implements auth.Provider.
func (p Provider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	serviceAccount corev1.ServiceAccount, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	// Check if a workload identity provider is specified in the service account.
	// If so, the current cluster is not GKE and the audience is the provider itself.
	audience, err := getWorkloadIdentityProviderAudience(serviceAccount)
	if err != nil {
		return nil, err
	}

	// Assume we are in GKE. In this case, retrieve the audience from the metadata.
	if audience == "" {
		audience, err = gkeMetadata.getAudience(ctx)
		if err != nil {
			return nil, err
		}
	}

	conf := externalaccount.Config{
		UniverseDomain:       "googleapis.com",
		Audience:             audience,
		SubjectTokenType:     "urn:ietf:params:oauth:token-type:jwt",
		TokenURL:             "https://sts.googleapis.com/v1/token",
		SubjectTokenSupplier: StaticTokenSupplier(oidcToken),
		Scopes:               scopes,
	}

	email, err := getServiceAccountEmail(serviceAccount)
	if err != nil {
		return nil, err
	}

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
func (Provider) GetAccessTokenOptionsForArtifactRepository(string) ([]auth.Option, error) {
	// GCP does not require any special options to retrieve access tokens.
	return nil, nil
}

const registryPattern = `^(((.+\.)?gcr\.io)|(.+-docker\.pkg\.dev))$`

var registryRegex = regexp.MustCompile(registryPattern)

// ParseArtifactRepository implements auth.Provider.
func (Provider) ParseArtifactRepository(artifactRepository string) (string, error) {
	registry, err := auth.GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		return "", err
	}

	if !registryRegex.MatchString(registry) {
		return "", fmt.Errorf("invalid GCP registry: '%s'. must match %s",
			registry, registryPattern)
	}

	// The artifact repository is irrelevant for issuing GCP registry credentials,
	// just return the provider name for inclusion in the cache key.
	return ProviderName, nil
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
		if err := parseCluster(cluster); err != nil {
			return nil, err
		}

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

		// Compare specified address and address from the cluster resource.
		endpoint := clusterResource.Endpoint
		if host != "" {
			canonicalAddress, err := auth.ParseClusterAddress(host)
			if err != nil {
				return nil, fmt.Errorf("failed to parse specified cluster address '%s': %w", host, err)
			}
			canonicalEndpoint, err := auth.ParseClusterAddress(endpoint)
			if err != nil {
				return nil, fmt.Errorf("failed to parse GKE endpoint '%s': %w", endpoint, err)
			}
			if canonicalAddress != canonicalEndpoint {
				return nil, fmt.Errorf("GKE endpoint '%s' does not match specified address: '%s'", endpoint, host)
			}
		}

		// Update host and CA with cluster details.
		host = endpoint
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
