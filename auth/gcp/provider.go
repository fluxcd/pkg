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
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google/externalaccount"
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

// NewDefaultToken implements auth.Provider.
func (p Provider) NewDefaultToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {
	var o auth.Options
	o.Apply(opts...)

	if hc := o.GetHTTPClient(); hc != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, hc)
	}

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

// GetAudience implements auth.Provider.
func (Provider) GetAudience(ctx context.Context) (string, error) {
	return gkeMetadata.workloadIdentityPool(ctx)
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

	audience, err := gkeMetadata.getAudience(ctx)
	if err != nil {
		return nil, err
	}

	conf := externalaccount.Config{
		UniverseDomain:       "googleapis.com",
		Audience:             audience,
		SubjectTokenType:     "urn:ietf:params:oauth:token-type:jwt",
		TokenURL:             "https://sts.googleapis.com/v1/token",
		SubjectTokenSupplier: TokenSupplier(oidcToken),
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

	if hc := o.GetHTTPClient(); hc != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, hc)
	}

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

// GetArtifactCacheKey implements auth.Provider.
func (Provider) GetArtifactCacheKey(artifactRepository string) string {
	// The artifact repository is irrelevant for GCP registry credentials.
	return ProviderName
}

// NewArtifactRegistryToken implements auth.Provider.
func (Provider) NewArtifactRegistryToken(ctx context.Context, artifactRepository string,
	accessToken auth.Token, opts ...auth.Option) (auth.Token, error) {

	t := accessToken.(*Token)

	// The artifact repository is irrelevant for GCP registry credentials.
	return &auth.ArtifactRegistryCredentials{
		Username:  "oauth2accesstoken",
		Password:  t.AccessToken,
		ExpiresAt: t.Expiry,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}
