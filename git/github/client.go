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
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/go-github/v81/github"
	"golang.org/x/net/http/httpproxy"

	"github.com/fluxcd/pkg/cache"
)

const (
	KeyAppID                = "githubAppID"
	KeyAppInstallationOwner = "githubAppInstallationOwner"
	KeyAppInstallationID    = "githubAppInstallationID"
	KeyAppPrivateKey        = "githubAppPrivateKey"
	KeyAppBaseURL           = "githubAppBaseURL"

	AccessTokenUsername = "x-access-token"
)

// Client is an authentication provider for GitHub Apps.
type Client struct {
	appID             int64
	installationOwner string
	installationID    int64
	privateKey        []byte
	rsaKey            *rsa.PrivateKey
	apiURL            string
	proxyURL          *url.URL
	httpClient        *http.Client
	cache             *cache.TokenCache
	kind              string
	name              string
	namespace         string
	operation         string
	tlsConfig         *tls.Config
	reflectSlug       bool
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
	if p.tlsConfig != nil {
		transport.TLSClientConfig = p.tlsConfig
	}
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

	if p.appID == 0 {
		return nil, fmt.Errorf("app ID must be provided to use github app authentication")
	}

	if p.installationOwner == "" && p.installationID == 0 {
		return nil, fmt.Errorf("app installation owner or ID must be provided to use github app authentication")
	}
	if p.installationOwner != "" && p.installationID != 0 {
		return nil, fmt.Errorf("only one of app installation owner or ID must be provided to use github app authentication")
	}

	if len(p.privateKey) == 0 {
		return nil, fmt.Errorf("private key must be provided to use github app authentication")
	}

	// Parse and store the private key
	rsaKey, err := jwt.ParseRSAPrivateKeyFromPEM(p.privateKey)
	if err != nil {
		return nil, fmt.Errorf("could not parse private key: %w", err)
	}
	p.rsaKey = rsaKey

	p.httpClient = &http.Client{Transport: transport}

	return p, nil
}

// WithTLSConfig sets the tls config to use with the transport.
func WithTLSConfig(tlsConfig *tls.Config) OptFunc {
	return func(p *Client) {
		p.tlsConfig = tlsConfig
	}
}

// WithAppData configures the client using data from a map.
// Note: appID and installationID parsing errors are deferred to New().
func WithAppData(appData map[string][]byte) OptFunc {
	return func(p *Client) {
		if val, ok := appData[KeyAppID]; ok {
			p.appID, _ = strconv.ParseInt(string(val), 10, 64)
		}
		if val, ok := appData[KeyAppInstallationOwner]; ok {
			p.installationOwner = string(val)
		}
		if val, ok := appData[KeyAppInstallationID]; ok {
			p.installationID, _ = strconv.ParseInt(string(val), 10, 64)
		}
		if val, ok := appData[KeyAppPrivateKey]; ok {
			p.privateKey = val
		}
		if val, ok := appData[KeyAppBaseURL]; ok {
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

// WithAppSlugReflection enables reflecting the app slug in the AppToken.
func WithAppSlugReflection() OptFunc {
	return func(p *Client) {
		p.reflectSlug = true
	}
}

// AppToken contains a GitHub App installation token and its expiry.
type AppToken struct {
	Token     string    `json:"token"`
	Slug      string    `json:"slug,omitempty"`
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
		return p.createInstallationToken(ctx)
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

// createJWT creates a JWT for GitHub App authentication.
func (p *Client) createJWT() (string, error) {
	// Truncate to seconds - GitHub rejects fractional timestamps
	now := time.Now().Truncate(time.Second)
	iat := now.Add(-30 * time.Second) // Clock drift allowance
	exp := iat.Add(2 * time.Minute)   // Short-lived JWT (only used to get installation token)
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(iat),
		ExpiresAt: jwt.NewNumericDate(exp),
		Issuer:    strconv.FormatInt(p.appID, 10),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(p.rsaKey)
}

// createInstallationToken creates an installation access token using the GitHub API.
func (p *Client) createInstallationToken(ctx context.Context) (*AppToken, error) {
	jwtToken, err := p.createJWT()
	if err != nil {
		return nil, err
	}

	// Create a GitHub client authenticated with the JWT
	ghClient := github.NewClient(p.httpClient).WithAuthToken(jwtToken)

	// Set custom base URL for GitHub Enterprise
	if p.apiURL != "" {
		apiURL := p.apiURL
		if !strings.HasSuffix(apiURL, "/") {
			apiURL += "/"
		}
		baseURL, err := url.Parse(apiURL)
		if err != nil {
			return nil, fmt.Errorf("invalid API URL: %w", err)
		}
		ghClient.BaseURL = baseURL
	}

	// Reflect the app slug in the token if enabled.
	var slug string
	if p.reflectSlug {
		app, _, err := ghClient.Apps.Get(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("failed to get app information: %w", err)
		}
		slug = app.GetSlug()
	}

	// Create the installation token
	installationID, err := p.getInstallationID(ctx, ghClient)
	if err != nil {
		return nil, err
	}
	token, _, err := ghClient.Apps.CreateInstallationToken(ctx, installationID, nil)
	if err != nil {
		return nil, err
	}

	return &AppToken{
		Token:     token.GetToken(),
		Slug:      slug,
		ExpiresAt: token.GetExpiresAt().Time,
	}, nil
}

// getInstallationID gets the installation ID for creating installation tokens.
// If an ID is already set on the client, it is returned directly. Otherwise,
// the installation ID is looked up using the installation owner.
func (p *Client) getInstallationID(ctx context.Context, ghClient *github.Client) (int64, error) {
	if p.installationID != 0 {
		return p.installationID, nil
	}

	var errs []error

	// Attempt owner as organization.
	orgInstallation, _, err := ghClient.Apps.FindOrganizationInstallation(ctx, p.installationOwner)
	if err == nil {
		return orgInstallation.GetID(), nil
	}
	errs = append(errs, fmt.Errorf("failed to find organization installation: %w", err))

	// Attempt owner as user.
	userInstallation, _, err := ghClient.Apps.FindUserInstallation(ctx, p.installationOwner)
	if err == nil {
		return userInstallation.GetID(), nil
	}
	errs = append(errs, fmt.Errorf("failed to find user installation: %w", err))

	return 0, errors.Join(errs...)
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
		fmt.Sprintf("%s=%d", KeyAppID, p.appID),
		fmt.Sprintf("%s=%s", KeyAppInstallationOwner, p.installationOwner),
		fmt.Sprintf("%s=%d", KeyAppInstallationID, p.installationID),
		fmt.Sprintf("%s=%s", KeyAppBaseURL, p.apiURL),
		fmt.Sprintf("%s=%s", KeyAppPrivateKey, string(p.privateKey)),
		fmt.Sprintf("reflectSlug=%v", p.reflectSlug),
	}
	rawKey := strings.Join(keyParts, ",")
	hash := sha256.Sum256([]byte(rawKey))
	return fmt.Sprintf("%x", hash)
}
