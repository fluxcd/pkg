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

package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/golang-jwt/jwt/v5"
	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/auth"
)

// ProviderName is the name of the Azure authentication provider.
const ProviderName = "azure"

// Provider implements the auth.Provider interface for Azure authentication.
type Provider struct{ Implementation }

// GetName implements auth.Provider.
func (Provider) GetName() string {
	return ProviderName
}

// NewDefaultToken implements auth.Provider.
func (p Provider) NewDefaultToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	var azOpts azidentity.DefaultAzureCredentialOptions

	if hc := o.GetHTTPClient(); hc != nil {
		azOpts.Transport = hc
	}

	cred, err := p.impl().NewDefaultAzureCredential(azOpts)
	if err != nil {
		return nil, err
	}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: getScopes(&o),
	})
	if err != nil {
		return nil, err
	}

	return &Token{token}, nil
}

// GetAudience implements auth.Provider.
func (Provider) GetAudience(context.Context) (string, error) {
	return "api://AzureADTokenExchange", nil
}

// GetIdentity implements auth.Provider.
func (Provider) GetIdentity(serviceAccount corev1.ServiceAccount) (string, error) {
	return getIdentity(serviceAccount)
}

// NewTokenForServiceAccount implements auth.Provider.
func (p Provider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	serviceAccount corev1.ServiceAccount, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	identity, err := getIdentity(serviceAccount)
	if err != nil {
		return nil, err
	}
	s := strings.Split(identity, "/")
	tenantID, clientID := s[0], s[1]

	azOpts := &azidentity.ClientAssertionCredentialOptions{}

	if hc := o.GetHTTPClient(); hc != nil {
		azOpts.Transport = hc
	}

	cred, err := p.impl().NewClientAssertionCredential(tenantID, clientID, func(context.Context) (string, error) {
		return oidcToken, nil
	}, azOpts)
	if err != nil {
		return nil, err
	}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: getScopes(&o),
	})
	if err != nil {
		return nil, err
	}

	return &Token{token}, nil
}

// GetArtifactCacheKey implements auth.Provider.
func (Provider) GetArtifactCacheKey(artifactRepository string) string {
	return getACRHost(artifactRepository)
}

// NewArtifactRegistryToken implements auth.Provider.
func (p Provider) NewArtifactRegistryToken(ctx context.Context, artifactRepository string,
	accessToken auth.Token, opts ...auth.Option) (auth.Token, error) {

	t := accessToken.(*Token)

	var o auth.Options
	o.Apply(opts...)

	// Build request.
	registryURL := artifactRepository
	if !strings.HasPrefix(artifactRepository, "http") {
		registryURL = fmt.Sprintf("https://%s", getACRHost(artifactRepository))
	}
	exchangeURL, err := url.Parse(registryURL)
	if err != nil {
		return nil, err
	}
	exchangeURL.Path = path.Join(exchangeURL.Path, "oauth2/exchange")
	parameters := url.Values{}
	parameters.Add("grant_type", "access_token")
	parameters.Add("service", exchangeURL.Hostname())
	parameters.Add("access_token", t.Token)
	body := strings.NewReader(parameters.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, exchangeURL.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send request.
	httpClient := http.DefaultClient
	if hc := o.GetHTTPClient(); hc != nil {
		httpClient = hc
	}
	resp, err := p.impl().SendRequest(req, httpClient)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Parse response.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status from ACR exchange request: %d", resp.StatusCode)
	}
	var tokenResp struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}
	var claims jwt.MapClaims
	if _, _, err := jwt.NewParser().ParseUnverified(tokenResp.RefreshToken, &claims); err != nil {
		return nil, err
	}
	expiry, err := claims.GetExpirationTime()
	if err != nil {
		return nil, err
	}

	return &auth.ArtifactRegistryCredentials{
		// https://docs.microsoft.com/en-us/azure/container-registry/container-registry-authentication?tabs=azure-cli#az-acr-login-with---expose-token
		Username:  "00000000-0000-0000-0000-000000000000",
		Password:  tokenResp.RefreshToken,
		ExpiresAt: expiry.Time,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}
