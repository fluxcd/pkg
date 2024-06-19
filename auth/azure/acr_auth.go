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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-containerregistry/pkg/authn"
)

// GetACRAuthConfig returns an AuthConfig that contains the credentials
// required to authenticate against ACR to access the provided image.
func (p *Provider) GetACRAuthConfig(ctx context.Context, registry string) (authn.AuthConfig, time.Duration, error) {
	var authConfig authn.AuthConfig
	var expiresIn time.Duration

	armToken, err := p.GetResourceManagerToken(ctx)
	if err != nil {
		return authConfig, expiresIn, err
	}

	ex := newExchanger(registry)
	refreshToken, err := ex.ExchangeACRAccessToken(string(armToken.Token))
	if err != nil {
		return authConfig, expiresIn, fmt.Errorf("failed to exchange token: %w", err)
	}

	authConfig = authn.AuthConfig{
		// This is the acr username used by Azure
		// See documentation: https://docs.microsoft.com/en-us/azure/container-registry/container-registry-authentication?tabs=azure-cli#az-acr-login-with---expose-token
		Username: "00000000-0000-0000-0000-000000000000",
		Password: refreshToken,
	}
	expiresIn, err = getExpirationFromJWT(refreshToken)
	if err != nil {
		return authConfig, expiresIn, fmt.Errorf("failed to determine token cache ttl: %w", err)
	}

	return authConfig, expiresIn, nil
}

// GetScopeProiderOption returns the cloud configuration based on the registry URL.
// List from https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/containers/azcontainerregistry/cloud_config.go#L16
func GetScopeProiderOption(url string) ProviderOptFunc {
	switch {
	case strings.HasSuffix(url, ".azurecr.cn"):
		return WithAzureChinaScope()
	case strings.HasSuffix(url, ".azurecr.us"):
		return WithAzureGovtScope()
	default:
		return nil
	}
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Resource     string `json:"resource"`
	TokenType    string `json:"token_type"`
}

type acrError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type exchanger struct {
	endpoint string
}

func newExchanger(endpoint string) *exchanger {
	return &exchanger{
		endpoint: endpoint,
	}
}

// ExchangeACRAccessToken exchanges an access token for a refresh token with the
// exchange service.
func (e *exchanger) ExchangeACRAccessToken(armToken string) (string, error) {
	// If the endpoint doesn't have a scheme, then prepend the "https" scheme.
	// This is required because the net/url package cannot parse an URL properly
	// without a scheme causing issues such as returning an URL object with an
	// empty host.
	if !(strings.HasPrefix(e.endpoint, "https://") || strings.HasPrefix(e.endpoint, "http://")) {
		e.endpoint = fmt.Sprintf("https://%s", e.endpoint)
	}

	// Construct the exchange URL.
	exchangeURL, err := url.Parse(e.endpoint)
	if err != nil {
		return "", err
	}
	exchangeURL.Path = "oauth2/exchange"

	parameters := url.Values{}
	parameters.Add("grant_type", "access_token")
	parameters.Add("service", exchangeURL.Hostname())
	parameters.Add("access_token", armToken)

	resp, err := http.PostForm(exchangeURL.String(), parameters)
	if err != nil {
		return "", fmt.Errorf("failed to send token exchange request: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read the body of the response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// Parse the error response.
		var errors []acrError
		if err = json.Unmarshal(b, &errors); err == nil {
			return "", fmt.Errorf("unexpected status code %d from exchange request: %s",
				resp.StatusCode, errors)
		}

		// Error response could not be parsed, return a generic error.
		return "", fmt.Errorf("unexpected status code %d from exchange request, response body: %s",
			resp.StatusCode, string(b))
	}

	var tokenResp tokenResponse
	if err = json.Unmarshal(b, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode the response: %w, response body: %s", err, string(b))
	}
	return tokenResp.RefreshToken, nil
}

// getExpirationFromJWT decodes the provided JWT and returns value
// of the `exp` key from the token claims.
func getExpirationFromJWT(tokenString string) (time.Duration, error) {
	parser := jwt.NewParser()
	// we don't care about verifying the JWT, we just want to extract the `exp`
	// attribute from the token.
	token, _, err := parser.ParseUnverified(tokenString, &jwt.RegisteredClaims{})
	if err != nil {
		return 0, err
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok {
		if claims.ExpiresAt != nil {
			expiration := claims.ExpiresAt.Time.Sub(time.Now())
			return expiration, nil
		}
	}

	return 0, errors.New("failed to extract expiration time from JWT")
}
