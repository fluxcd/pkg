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
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"golang.org/x/net/http/httpproxy"

	"github.com/fluxcd/pkg/cache"
)

const (
	KeyAppID             = "githubAppID"
	KeyAppInstallationID = "githubAppInstallationID"
	KeyAppPrivateKey     = "githubAppPrivateKey"
	KeyAppBaseURL        = "githubAppBaseURL"

	AccessTokenUsername = "x-access-token"
)

// Client is an authentication provider for GitHub Apps.
type Client struct {
	appID          string
	installationID string
	privateKey     []byte
	apiURL         string
	proxyURL       *url.URL
	ghTransport    *ghinstallation.Transport
	cache          *cache.TokenCache
	kind           string
	name           string
	namespace      string
	operation      string
}

// OptFunc enables specifying options for the provider.
type OptFunc func(*Client)

// New returns a new authentication provider for GitHub Apps.
func New(opts ...OptFunc) (*Client, error) {
	p := &Client{}
	for _, opt := range opts {
		opt(p)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if p.proxyURL != nil {
		proxyStr := p.proxyURL.String()
		proxyConfig := &httpproxy.Config{
			HTTPProxy:  proxyStr,
			HTTPSProxy: proxyStr,
		}
		proxyFunc := func(req *http.Request) (*url.URL, error) {
			return proxyConfig.ProxyFunc()(req.URL)
		}
		transport.Proxy = proxyFunc
	}

	if len(p.appID) == 0 {
		return nil, fmt.Errorf("app ID must be provided to use github app authentication")
	}
	appID, err := strconv.Atoi(p.appID)
	if err != nil {
		return nil, fmt.Errorf("invalid app id, err: %w", err)
	}

	if len(p.installationID) == 0 {
		return nil, fmt.Errorf("app installation ID must be provided to use github app authentication")
	}
	installationID, err := strconv.Atoi(p.installationID)
	if err != nil {
		return nil, fmt.Errorf("invalid app installation id, err: %w", err)
	}

	if len(p.privateKey) == 0 {
		return nil, fmt.Errorf("private key must be provided to use github app authentication")
	}

	p.ghTransport, err = ghinstallation.New(transport, int64(appID), int64(installationID), p.privateKey)
	if err != nil {
		return nil, err
	}

	if p.apiURL != "" {
		p.ghTransport.BaseURL = p.apiURL
	}

	return p, nil
}

// WithAppData configures the client using data from a map
func WithAppData(appData map[string][]byte) OptFunc {
	return func(p *Client) {
		val, ok := appData[KeyAppID]
		if ok {
			p.appID = string(val)
		}
		val, ok = appData[KeyAppInstallationID]
		if ok {
			p.installationID = string(val)
		}
		val, ok = appData[KeyAppPrivateKey]
		if ok {
			p.privateKey = val
		}
		val, ok = appData[KeyAppBaseURL]
		if ok {
			p.apiURL = string(val)
		}
	}
}

// WithProxyURL sets the proxy URL to use with the transport.
func WithProxyURL(proxyURL *url.URL) OptFunc {
	return func(p *Client) {
		p.proxyURL = proxyURL
	}
}

// WithCache sets the token cache and the object involved in the operation for
// recording cache events.
func WithCache(cache *cache.TokenCache, kind, name, namespace, operation string) OptFunc {
	return func(p *Client) {
		p.cache = cache
		p.kind = kind
		p.name = name
		p.namespace = namespace
		p.operation = operation
	}
}

// AppToken contains a GitHub App installation token and its expiry.
type AppToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// GetDuration returns the duration until the token expires.
func (at *AppToken) GetDuration() time.Duration {
	return time.Until(at.ExpiresAt)
}

// GetToken returns the token that can be used to authenticate
// as a GitHub App installation.
// Ref: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/authenticating-as-a-github-app-installation
func (p *Client) GetToken(ctx context.Context) (*AppToken, error) {
	newToken := func(ctx context.Context) (cache.Token, error) {
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

	if p.cache == nil {
		token, err := newToken(ctx)
		if err != nil {
			return nil, err
		}
		return token.(*AppToken), nil
	}

	var opts []cache.Options
	if p.kind != "" && p.name != "" && p.namespace != "" && p.operation != "" {
		opts = append(opts, cache.WithInvolvedObject(p.kind, p.name, p.namespace, p.operation))
	}

	token, _, err := p.cache.GetOrSet(ctx, p.buildCacheKey(), newToken, opts...)
	if err != nil {
		return nil, err
	}
	return token.(*AppToken), nil
}

// GetCredentials returns the GitHub App installation username and password
// for authenticating Git operations.
func GetCredentials(ctx context.Context, opts ...OptFunc) (string, string, error) {
	client, err := New(opts...)
	if err != nil {
		return "", "", err
	}
	appToken, err := client.GetToken(ctx)
	if err != nil {
		return "", "", err
	}
	return AccessTokenUsername, appToken.Token, nil
}

func (p *Client) buildCacheKey() string {
	keyParts := []string{
		fmt.Sprintf("%s=%s", KeyAppID, p.appID),
		fmt.Sprintf("%s=%s", KeyAppInstallationID, p.installationID),
		fmt.Sprintf("%s=%s", KeyAppBaseURL, p.apiURL),
		fmt.Sprintf("%s=%s", KeyAppPrivateKey, string(p.privateKey)),
	}
	rawKey := strings.Join(keyParts, ",")
	hash := sha256.Sum256([]byte(rawKey))
	return fmt.Sprintf("%x", hash)
}
