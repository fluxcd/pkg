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
	corev1 "k8s.io/api/core/v1"

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
func (Provider) GetAudiences(context.Context, corev1.ServiceAccount) ([]string, error) {
	// Use TokenRequest default audiences.
	return nil, nil
}

// GetIdentity implements auth.RESTConfigProvider.
func (Provider) GetIdentity(serviceAccount corev1.ServiceAccount) (string, error) {
	return fmt.Sprintf("system:serviceaccount:%s:%s", serviceAccount.Namespace, serviceAccount.Name), nil
}

// NewTokenForServiceAccount implements auth.RESTConfigProvider.
func (Provider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	serviceAccount corev1.ServiceAccount, opts ...auth.Option) (auth.Token, error) {

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
