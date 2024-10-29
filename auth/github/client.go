/*
Copyright 2024 The Flux authors

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
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	corev1 "k8s.io/api/core/v1"
)

const (
	AppIDKey             = "githubAppID"
	AppInstallationIDKey = "githubAppInstallationID"
	AppPrivateKey        = "githubAppPrivateKey"
	ApiURLKey            = "githubApiURL"
)

// Client is an authentication provider for GitHub Apps.
type Client struct {
	appID          int
	installationID int
	privateKey     []byte
	apiURL         string
	proxyURL       *url.URL
	ghTransport    *ghinstallation.Transport
}

// OptFunc enables specifying options for the provider.
type OptFunc func(*Client) error

// New returns a new authentication provider for GitHub Apps.
func New(opts ...OptFunc) (*Client, error) {
	var err error

	p := &Client{}
	for _, opt := range opts {
		err = opt(p)
		if err != nil {
			return nil, err
		}
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if p.proxyURL != nil {
		transport.Proxy = http.ProxyURL(p.proxyURL)
	}
	p.ghTransport, err = ghinstallation.New(transport, int64(p.appID), int64(p.installationID), p.privateKey)
	if err != nil {
		return nil, err
	}

	if p.apiURL != "" {
		p.ghTransport.BaseURL = p.apiURL
	}

	return p, nil
}

// WithInstallationID configures the installation ID of the GitHub App.
func WithInstllationID(installationID int) OptFunc {
	return func(p *Client) error {
		p.installationID = installationID
		return nil
	}
}

// WithAppID configures the app ID of the GitHub App.
func WithAppID(appID int) OptFunc {
	return func(p *Client) error {
		p.appID = appID
		return nil
	}
}

// WithPrivateKey configures the private key of the GitHub App.
func WithPrivateKey(pk []byte) OptFunc {
	return func(p *Client) error {
		p.privateKey = pk
		return nil
	}
}

// WithApiURL configures the GitHub API endpoint to use to fetch GitHub App
// installation token
func WithApiURL(apiURL string) OptFunc {
	return func(p *Client) error {
		p.apiURL = apiURL
		return nil
	}
}

// WithSecret configures the client using the Kubernetes Secret.
func WithSecret(secret corev1.Secret) OptFunc {
	return func(p *Client) error {
		var err error
		for _, key := range []string{AppIDKey, AppInstallationIDKey, AppPrivateKey} {
			if _, exists := secret.Data[key]; !exists {
				return fmt.Errorf("github app secret must contain key : %s", key)
			}
		}
		p.appID, err = strconv.Atoi(string(secret.Data[AppIDKey]))
		if err != nil {
			return fmt.Errorf("github app secret data error for key : %s, err: %v", AppIDKey, err)
		}
		p.installationID, err = strconv.Atoi(string(secret.Data[AppInstallationIDKey]))
		if err != nil {
			return fmt.Errorf("github app secret data error for key : %s, err: %v", AppInstallationIDKey, err)
		}
		p.privateKey = secret.Data[AppPrivateKey]
		p.apiURL = string(secret.Data[ApiURLKey])
		return nil
	}
}

// WithProxyURL sets the proxy URL to use with the transport
func WithProxyURL(proxyURL *url.URL) OptFunc {
	return func(p *Client) error {
		p.proxyURL = proxyURL
		return nil
	}
}

// AppToken contains a GitHub App installation token and its expiry.
type AppToken struct {
	Token     string
	ExpiresAt time.Time
}

// GetToken returns the token that can be used to authenticate
// as a GitHub App installation.
// Ref: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation
func (p *Client) GetToken(ctx context.Context) (*AppToken, error) {
	token, err := p.ghTransport.Token(ctx)
	if err != nil {
		return nil, err
	}

	expiresAt, _, err := p.ghTransport.Expiry()
	if err != nil {
		return nil, err
	}

	return &AppToken{
		Token:     token,
		ExpiresAt: expiresAt,
	}, nil
}
