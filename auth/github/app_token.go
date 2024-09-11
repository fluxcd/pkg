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

package github

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	corev1 "k8s.io/api/core/v1"
)

const (
	AppIDKey             = "githubAppID"
	AppInstallationIDKey = "githubAppInstallationID"
	AppPkKey             = "githubAppPrivateKey"
	ApiURLKey            = "githubApiURL"
)

// Provider is an authentication provider for GitHub Apps.
type Provider struct {
	apiURL         string
	privateKey     []byte
	appID          int
	installationID int
	transport      http.RoundTripper
}

// ProviderOptFunc enables specifying options for the provider.
type ProviderOptFunc func(*Provider) error

// NewProvider returns a new authentication provider for GitHub Apps.
func NewProvider(opts ...ProviderOptFunc) (*Provider, error) {
	p := &Provider{}
	for _, opt := range opts {
		err := opt(p)
		if err != nil {
			return nil, err
		}
	}
	return p, nil
}

// WithInstllationID configures the installation ID of the GitHub App.
func WithInstllationID(installationID int) ProviderOptFunc {
	return func(p *Provider) error {
		p.installationID = installationID
		return nil
	}
}

// WithAppID configures the app ID of the GitHub App.
func WithAppID(appID int) ProviderOptFunc {
	return func(p *Provider) error {
		p.appID = appID
		return nil
	}
}

// WithPrivateKey configures the private key related to the GitHub App.
func WithPrivateKey(pk []byte) ProviderOptFunc {
	return func(p *Provider) error {
		p.privateKey = pk
		return nil
	}
}

// WithApiURL configures the API endpoint to use for fetching the token
// related to the GitHub App.
func WithApiURL(apiURL string) ProviderOptFunc {
	return func(p *Provider) error {
		p.apiURL = apiURL
		return nil
	}
}

// WithTransport configures the HTTP transport to use while making API calls.
func WithTransport(t http.RoundTripper) ProviderOptFunc {
	return func(p *Provider) error {
		p.transport = t
		return nil
	}
}

// WithSecret configures the provider using the data present in the provided
// Kubernetes Secret.
func WithSecret(secret corev1.Secret) ProviderOptFunc {
	return func(p *Provider) error {
		var err error
		p.appID, err = strconv.Atoi(string(secret.Data[AppIDKey]))
		if err != nil {
			return err
		}
		p.installationID, err = strconv.Atoi(string(secret.Data[AppInstallationIDKey]))
		if err != nil {
			return err
		}
		p.privateKey = secret.Data[AppPkKey]
		p.apiURL = string(secret.Data[ApiURLKey])
		return nil
	}
}

// AppToken contains a GitHub App instllation token and its TTL.
type AppToken struct {
	Token     string
	ExpiresIn time.Duration
}

// GetAppToken returns the token that can be used to authenticate
// as a GitHub App installation.
// Ref: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation
func (p *Provider) GetAppToken(ctx context.Context) (*AppToken, error) {
	if p.transport == nil {
		p.transport = http.DefaultTransport
	}

	ghTransport, err := ghinstallation.New(p.transport, int64(p.appID), int64(p.installationID), p.privateKey)
	if err != nil {
		return nil, err
	}
	if p.apiURL != "" {
		ghTransport.BaseURL = p.apiURL
	}

	token, err := ghTransport.Token(ctx)
	if err != nil {
		return nil, err
	}
	expiresAt, _, err := ghTransport.Expiry()
	if err != nil {
		return nil, err
	}
	return &AppToken{
		Token:     token,
		ExpiresIn: expiresAt.UTC().Sub(time.Now().UTC()),
	}, nil
}
