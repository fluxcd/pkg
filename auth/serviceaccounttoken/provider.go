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

package serviceaccounttoken

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/fluxcd/pkg/auth"
)

// ProviderName is the name of the provider implemented by this package.
// Only the Kustomization and HelmRelease APIs refer to this package as
// a provider for historical reasons. New APIs should refer to it as the
// ServiceAccountToken credential provider (see CredentialName).
const ProviderName = "generic"

// CredentialName is the name of the credential type implemented by this package.
const CredentialName = "ServiceAccountToken"

// Provider implements the auth.Provider interface for generic authentication.
type Provider struct{ Implementation }

// Ensure Provider implements the expected interfaces.
var _ auth.Provider = Provider{}
var _ auth.ProviderWithOIDCImpersonation = Provider{}
var _ auth.RESTConfigProvider = Provider{}
var _ auth.ArtifactRegistryCredentialsProvider = Provider{}

// GetName implements auth.RESTConfigProvider.
func (p Provider) GetName() string {
	return CredentialName
}

// NewControllerToken implements auth.RESTConfigProvider.
func (p Provider) NewControllerToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	if o.Client == nil {
		return nil, errors.New("client is required to create a controller token")
	}

	// Like all providers, this one should fetch controller-level credentials
	// from the environment. In this case, this means opening the well-known
	// Kubernetes service account token file and parsing it to figure out
	// the controller's identity.
	saRef, err := auth.FindPodServiceAccount(p.impl().ReadFile)
	if err != nil {
		return nil, err
	}
	token, err := auth.CreateServiceAccountToken(ctx, o.Client, *saRef, o.Audiences...)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create kubernetes token for controller service account '%s': %w",
			saRef.String(), err)
	}

	exp, err := getExpirationFromToken(token)
	if err != nil {
		return nil, err
	}

	return &Token{
		Token:     token,
		ExpiresAt: *exp,
	}, nil
}

// GetAudiences implements auth.RESTConfigProvider.
func (Provider) GetAudiences(_ context.Context, opts ...auth.Option) (string, string, error) {
	var o auth.Options
	o.Apply(opts...)
	if len(o.Audiences) > 0 {
		return o.Audiences[0], "", nil
	}
	// Use TokenRequest API default audiences.
	return "", "", nil
}

// GetIdentity implements auth.RESTConfigProvider.
func (Provider) GetIdentity(opts ...auth.Option) (auth.Identity, error) {
	var o auth.Options
	o.Apply(opts...)
	if o.ServiceAccount == nil {
		return nil, auth.ErrNoIdentityForOIDCImpersonation
	}
	return &Identity{
		Name:      o.ServiceAccount.Name,
		Namespace: o.ServiceAccount.Namespace,
	}, nil
}

// NewTokenForOIDCToken implements auth.RESTConfigProvider.
func (Provider) NewTokenForOIDCToken(ctx context.Context, oidcToken, _ string,
	_ auth.Identity, opts ...auth.Option) (auth.Token, error) {

	exp, err := getExpirationFromToken(oidcToken)
	if err != nil {
		return nil, err
	}

	return &Token{
		Token:     oidcToken,
		ExpiresAt: *exp,
	}, nil
}

// GetAccessTokenOptionsForArtifactRepository implements auth.ArtifactRegistryCredentialsProvider.
func (Provider) GetAccessTokenOptionsForArtifactRepository(string) ([]auth.Option, error) {
	// No special options are needed to get an access token for artifact registry.
	return nil, nil
}

// ParseArtifactRepository implements auth.ArtifactRegistryCredentialsProvider.
func (p Provider) ParseArtifactRepository(artifactRepository string) (string, error) {
	// The artifact repository is irrelevant for issuing the ServiceAccount token,
	// just return the provider name for inclusion in the cache key.
	return p.GetName(), nil
}

// NewArtifactRegistryCredentials implements auth.ArtifactRegistryCredentialsProvider.
func (p Provider) NewArtifactRegistryCredentials(ctx context.Context, registryInput string,
	accessToken auth.Token, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {

	token := accessToken.(*Token)

	return &auth.ArtifactRegistryCredentials{
		Authenticator: &authn.Bearer{Token: token.Token},
		ExpiresAt:     token.ExpiresAt,
	}, nil
}

// GetAccessTokenOptionsForCluster implements auth.RESTConfigProvider.
func (Provider) GetAccessTokenOptionsForCluster(opts ...auth.Option) ([][]auth.Option, error) {

	var o auth.Options
	o.Apply(opts...)

	audiences := o.Audiences
	if len(audiences) == 0 {
		// Use cluster address as the default audience.
		audiences = []string{o.ClusterAddress}
	}

	return [][]auth.Option{{auth.WithAudiences(audiences...)}}, nil
}

// NewRESTConfig implements auth.RESTConfigProvider.
func (Provider) NewRESTConfig(ctx context.Context, accessTokens []auth.Token,
	opts ...auth.Option) (*auth.RESTConfig, error) {

	token := accessTokens[0].(*Token)

	var o auth.Options
	o.Apply(opts...)

	// Parse the cluster address.
	host := o.ClusterAddress
	if host == "" {
		return nil, errors.New("cluster address is required to create a REST config")
	}
	var err error
	host, err = auth.ParseClusterAddress(host)
	if err != nil {
		return nil, fmt.Errorf("failed to parse cluster address %s: %w", o.ClusterAddress, err)
	}

	// Get CA if provided.
	var caData []byte
	if o.CAData != "" {
		caData = []byte(o.CAData)
	}

	return &auth.RESTConfig{
		Host:        host,
		CAData:      caData,
		BearerToken: token.Token,
		ExpiresAt:   token.ExpiresAt,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}

func getExpirationFromToken(token string) (*time.Time, error) {
	tok, _, err := jwt.NewParser().ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse service account token: %w", err)
	}
	exp, err := tok.Claims.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get expiration time from service account token: %w", err)
	}
	return &exp.Time, nil
}
