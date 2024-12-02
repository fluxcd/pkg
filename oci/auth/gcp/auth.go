/*
Copyright 2022 The Flux authors

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
	"net/url"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/fluxcd/pkg/oci"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// ValidHost returns if a given host is a valid GCR host.
func ValidHost(host string) bool {
	return host == "gcr.io" || strings.HasSuffix(host, ".gcr.io") || strings.HasSuffix(host, "-docker.pkg.dev")
}

// Client is a GCP GCR client which can log into the registry and return
// authorization information.
type Client struct {
	proxyURL    *url.URL
	tokenSource oauth2.TokenSource
}

// Option is a functional option for configuring the client.
type Option func(*Client)

// WithProxyURL sets the proxy URL for the client.
func WithProxyURL(proxyURL *url.URL) Option {
	return func(c *Client) {
		c.proxyURL = proxyURL
	}
}

// WithTokenSource sets a custom token source for the client.
func (c *Client) WithTokenSource(ts oauth2.TokenSource) *Client {
	c.tokenSource = ts
	return c
}

// NewClient creates a new GCR client with default configurations.
func NewClient(opts ...Option) *Client {
	client := &Client{}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// getLoginAuth obtains authentication using the default GCP credential chain.
// This supports various authentication methods including service account JSON,
// external account JSON, user credentials, and GCE metadata service.
func (c *Client) getLoginAuth(ctx context.Context) (authn.AuthConfig, time.Time, error) {
	var authConfig authn.AuthConfig

	// Define the required scopes for accessing GCR.
	scopes := []string{"https://www.googleapis.com/auth/cloud-platform"}

	var tokenSource oauth2.TokenSource
	var err error

	// Use the injected token source if available; otherwise, use the default.
	if c.tokenSource != nil {
		tokenSource = c.tokenSource
	} else {
		// Obtain the default token source.
		tokenSource, err = google.DefaultTokenSource(ctx, scopes...)
		if err != nil {
			return authConfig, time.Time{}, fmt.Errorf("failed to get default token source: %w", err)
		}
	}

	// Retrieve the token.
	token, err := tokenSource.Token()
	if err != nil {
		return authConfig, time.Time{}, fmt.Errorf("failed to obtain token: %w", err)
	}

	// Set up the authentication configuration.
	authConfig = authn.AuthConfig{
		Username: "oauth2accesstoken",
		Password: token.AccessToken,
	}

	return authConfig, token.Expiry, nil
}

// Login attempts to get the authentication material for GCR.
// It returns the authentication material and the expiry time of the token.
// The caller can ensure that the passed image is a valid GCR image using ValidHost().
func (c *Client) LoginWithExpiry(ctx context.Context, autoLogin bool, image string, ref name.Reference) (authn.Authenticator, time.Time, error) {
	if autoLogin {
		logr.FromContextOrDiscard(ctx).Info("logging in to GCP GCR for " + image)
		authConfig, expiresAt, err := c.getLoginAuth(ctx)
		if err != nil {
			logr.FromContextOrDiscard(ctx).Info("error logging into GCP " + err.Error())
			return nil, time.Time{}, err
		}

		auth := authn.FromConfig(authConfig)
		return auth, expiresAt, nil
	}
	return nil, time.Time{}, fmt.Errorf("GCR authentication failed: %w", oci.ErrUnconfiguredProvider)
}

// Login attempts to get the authentication material for GCR. The caller can
// ensure that the passed image is a valid GCR image using ValidHost().
func (c *Client) Login(ctx context.Context, autoLogin bool, image string, ref name.Reference) (authn.Authenticator, error) {
	auth, _, err := c.LoginWithExpiry(ctx, autoLogin, image, ref)
	return auth, err
}

// OIDCLogin attempts to get the authentication material for GCR from the token url set in the client.
//
// Deprecated: Use LoginWithExpiry instead.
func (c *Client) OIDCLogin(ctx context.Context) (authn.Authenticator, error) {
	authConfig, _, err := c.getLoginAuth(ctx)
	if err != nil {
		logr.FromContextOrDiscard(ctx).Info("error logging into GCP " + err.Error())
		return nil, err
	}

	auth := authn.FromConfig(authConfig)
	return auth, nil
}
