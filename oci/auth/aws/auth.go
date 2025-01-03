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

package aws

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"

	"github.com/fluxcd/pkg/oci"
)

// This regex is sourced from the AWS ECR Credential Helper (https://github.com/awslabs/amazon-ecr-credential-helper).
// It covers both public AWS partitions like amazonaws.com, China partitions like amazonaws.com.cn, and non-public partitions.
var registryPartRe = regexp.MustCompile(`([0-9+]*).dkr.ecr(?:-fips)?\.([^/.]*)\.(amazonaws\.com[.cn]*|sc2s\.sgov\.gov|c2s\.ic\.gov|cloud\.adc-e\.uk|csp\.hci\.ic\.gov)`)

// ParseRegistry returns the AWS account ID and region and `true` if
// the image registry/repository is hosted in AWS's Elastic Container Registry,
// otherwise empty strings and `false`.
func ParseRegistry(registry string) (accountId, awsEcrRegion string, ok bool) {
	registryParts := registryPartRe.FindAllStringSubmatch(registry, -1)
	if len(registryParts) < 1 || len(registryParts[0]) < 3 {
		return "", "", false
	}
	return registryParts[0][1], registryParts[0][2], true
}

// Client is a AWS ECR client which can log into the registry and return
// authorization information.
type Client struct {
	config   *aws.Config
	mu       sync.Mutex
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

// NewClient creates a new empty ECR client.
// NOTE: In order to avoid breaking the auth API with aws-sdk-go-v2's default
// config, return an empty Client. Client.getLoginAuth() loads the default
// config if Client.config is nil. This also enables tests to configure the
// Client with stub before calling the login method using Client.WithConfig().
func NewClient(opts ...Option) *Client {
	client := &Client{}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// WithConfig allows setting the client config if it's uninitialized.
func (c *Client) WithConfig(cfg *aws.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.config == nil {
		c.config = cfg
	}
}

// getLoginAuth obtains authentication for ECR given the
// region (taken from the image). This assumes that the pod has
// IAM permissions to get an authentication token, which will usually
// be the case if it's running in EKS, and may need additional setup
// otherwise (visit https://aws.github.io/aws-sdk-go-v2/docs/configuring-sdk/
// as a starting point).
func (c *Client) getLoginAuth(ctx context.Context, awsEcrRegion string) (authn.AuthConfig, time.Time, error) {
	var authConfig authn.AuthConfig
	var cfg aws.Config

	c.mu.Lock()
	if c.config != nil {
		cfg = c.config.Copy()
	} else {
		var confOpts []func(*config.LoadOptions) error
		confOpts = append(confOpts, config.WithRegion(awsEcrRegion))
		if c.proxyURL != nil {
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.Proxy = http.ProxyURL(c.proxyURL)
			confOpts = append(confOpts, config.WithHTTPClient(&http.Client{Transport: transport}))
		}

		var err error
		cfg, err = config.LoadDefaultConfig(ctx, confOpts...)
		if err != nil {
			c.mu.Unlock()
			return authConfig, time.Time{}, fmt.Errorf("failed to load default configuration: %w", err)
		}
		c.config = &cfg
	}
	c.mu.Unlock()

	ecrService := ecr.NewFromConfig(cfg)
	// NOTE: ecr.GetAuthorizationTokenInput has deprecated RegistryIds. Hence,
	// pass nil input.
	ecrToken, err := ecrService.GetAuthorizationToken(ctx, nil)
	if err != nil {
		return authConfig, time.Time{}, err
	}

	// Validate the authorization data.
	if len(ecrToken.AuthorizationData) == 0 {
		return authConfig, time.Time{}, errors.New("no authorization data")
	}
	if ecrToken.AuthorizationData[0].AuthorizationToken == nil {
		return authConfig, time.Time{}, fmt.Errorf("no authorization token")
	}
	token, err := base64.StdEncoding.DecodeString(*ecrToken.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		return authConfig, time.Time{}, err
	}

	tokenSplit := strings.Split(string(token), ":")
	// Validate the tokens.
	if len(tokenSplit) != 2 {
		return authConfig, time.Time{}, fmt.Errorf("invalid authorization token, expected the token to have two parts separated by ':', got %d parts", len(tokenSplit))
	}
	authConfig = authn.AuthConfig{
		Username: tokenSplit[0],
		Password: tokenSplit[1],
	}
	expiresAt := ecrToken.AuthorizationData[0].ExpiresAt
	if expiresAt == nil {
		expiresAt = &time.Time{}
	}
	return authConfig, *expiresAt, nil
}

// LoginWithExpiry attempts to get the authentication material for ECR.
// It returns the authentication material and the expiry time of the token.
func (c *Client) LoginWithExpiry(ctx context.Context, autoLogin bool, image string) (authn.Authenticator, time.Time, error) {
	if autoLogin {
		logr.FromContextOrDiscard(ctx).Info("logging in to AWS ECR for " + image)
		_, awsEcrRegion, ok := ParseRegistry(image)
		if !ok {
			return nil, time.Time{}, errors.New("failed to parse AWS ECR image, invalid ECR image")
		}

		authConfig, expiresAt, err := c.getLoginAuth(ctx, awsEcrRegion)
		if err != nil {
			return nil, time.Time{}, err
		}

		auth := authn.FromConfig(authConfig)
		return auth, expiresAt, nil
	}
	return nil, time.Time{}, fmt.Errorf("ECR authentication failed: %w", oci.ErrUnconfiguredProvider)
}

// Login attempts to get the authentication material for ECR.
func (c *Client) Login(ctx context.Context, autoLogin bool, image string) (authn.Authenticator, error) {
	auth, _, err := c.LoginWithExpiry(ctx, autoLogin, image)
	return auth, err
}

// OIDCLogin attempts to get the authentication material for ECR.
//
// Deprecated: Use LoginWithExpiry instead.
func (c *Client) OIDCLogin(ctx context.Context, registryURL string) (authn.Authenticator, error) {
	_, awsEcrRegion, ok := ParseRegistry(registryURL)
	if !ok {
		return nil, errors.New("failed to parse AWS ECR image, invalid ECR image")
	}

	authConfig, _, err := c.getLoginAuth(ctx, awsEcrRegion)
	if err != nil {
		return nil, err
	}

	auth := authn.FromConfig(authConfig)
	return auth, nil
}
