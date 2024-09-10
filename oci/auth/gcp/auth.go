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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/fluxcd/pkg/oci"
)

type gceToken struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// GCP_TOKEN_URL is the default GCP metadata endpoint used for authentication.
const GCP_TOKEN_URL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

// ValidHost returns if a given host is a valid GCR host.
func ValidHost(host string) bool {
	return host == "gcr.io" || strings.HasSuffix(host, ".gcr.io") || strings.HasSuffix(host, "-docker.pkg.dev")
}

// Client is a GCP GCR client which can log into the registry and return
// authorization information.
type Client struct {
	tokenURL string
	proxyURL *url.URL
}

// Option is a functional option for configuring the client.
type Option func(*Client)

// WithProxyURL sets the proxy URL for the client.
func WithProxyURL(proxyURL *url.URL) Option {
	return func(c *Client) {
		c.proxyURL = proxyURL
	}
}

// NewClient creates a new GCR client with default configurations.
func NewClient(opts ...Option) *Client {
	client := &Client{tokenURL: GCP_TOKEN_URL}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// WithTokenURL sets the token URL used by the GCR client.
func (c *Client) WithTokenURL(url string) *Client {
	c.tokenURL = url
	return c
}

// getLoginAuth obtains authentication by getting a token from the metadata API
// on GCP. This assumes that the pod has right to pull the image which would be
// the case if it is hosted on GCP. It works with both service account and
// workload identity enabled clusters.
func (c *Client) getLoginAuth(ctx context.Context) (authn.AuthConfig, time.Time, error) {
	var authConfig authn.AuthConfig

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.tokenURL, nil)
	if err != nil {
		return authConfig, time.Time{}, err
	}

	request.Header.Add("Metadata-Flavor", "Google")

	var transport http.RoundTripper
	if c.proxyURL != nil {
		t := http.DefaultTransport.(*http.Transport).Clone()
		t.Proxy = http.ProxyURL(c.proxyURL)
		transport = t
	}

	client := &http.Client{Transport: transport}
	response, err := client.Do(request)
	if err != nil {
		return authConfig, time.Time{}, err
	}
	defer response.Body.Close()
	defer io.Copy(io.Discard, response.Body)

	if response.StatusCode != http.StatusOK {
		return authConfig, time.Time{}, fmt.Errorf("unexpected status from metadata service: %s", response.Status)
	}

	var accessToken gceToken
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&accessToken); err != nil {
		return authConfig, time.Time{}, err
	}

	authConfig = authn.AuthConfig{
		Username: "oauth2accesstoken",
		Password: accessToken.AccessToken,
	}

	// add expiresIn seconds to the current time to get the expiry time
	expiresAt := time.Now().Add(time.Duration(accessToken.ExpiresIn) * time.Second)

	return authConfig, expiresAt, nil
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
