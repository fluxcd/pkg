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

package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SERVICE_ACCOUNT_TOKEN_URL is the default GCP metadata server endpoint to
// fetch the access token for a GCP service account.
const SERVICE_ACCOUNT_TOKEN_URL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

// SERVICE_ACCOUNT_EMAIL_URL is the default GCP metadata server endpoint to
// fetch the email for a GCP service account.
const SERVICE_ACCOUNT_EMAIL_URL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/email"

// Provider is an authentication provider for GCP.
type Provider struct {
	tokenURL string
	emailURL string
}

// ProviderOptFunc enables specifying options for the provider.
type ProviderOptFunc func(*Provider)

// NewProvider returns a new authentication provider for GCP.
func NewProvider(opts ...ProviderOptFunc) *Provider {
	p := &Provider{}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// WithTokenURL configures the url that the provider should use
// to fetch the access token for a GCP service account.
func WithTokenURL(tokenURL string) ProviderOptFunc {
	return func(p *Provider) {
		p.tokenURL = tokenURL
	}
}

// WithEmailURL configures the url that the provider should use
// to fetch the email for a GCP service account.
func WithEmailURL(emailURL string) ProviderOptFunc {
	return func(p *Provider) {
		p.emailURL = emailURL
	}
}

// ServiceAccountToken is the object returned by the GKE metadata server
// upon requesting for a GCP service account token.
// Ref: https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity#metadata_server
type ServiceAccountToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// GetServiceAccountToken fetches the access token for the service account
// that the Pod is configured to run as, using Workload Identity.
// Ref: https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity
// The Kubernetes service account must be bound to a GCP service account with
// the appropriate permissions.
func (p *Provider) GetServiceAccountToken(ctx context.Context) (*ServiceAccountToken, error) {
	if p.tokenURL == "" {
		p.tokenURL = SERVICE_ACCOUNT_TOKEN_URL
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.tokenURL, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Add("Metadata-Flavor", "Google")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	defer io.Copy(io.Discard, response.Body)

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status from metadata service: %s", response.Status)
	}

	var accessToken ServiceAccountToken
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&accessToken); err != nil {
		return nil, err
	}

	return &accessToken, nil
}

// GetServiceAccountEmail fetches the email for the service account
// that the Pod is configured to run as, using Workload Identity.
// Ref: https://cloud.google.com/kubernetes-engine/docs/concepts/workload-identity
func (p *Provider) GetServiceAccountEmail(ctx context.Context) (string, error) {
	if p.emailURL == "" {
		p.emailURL = SERVICE_ACCOUNT_EMAIL_URL
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, p.emailURL, nil)
	if err != nil {
		return "", err
	}

	request.Header.Add("Metadata-Flavor", "Google")

	client := &http.Client{}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	defer io.Copy(io.Discard, response.Body)

	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status from metadata service: %s", response.Status)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}
