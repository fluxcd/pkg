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

package azure

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	_ "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/go-logr/logr"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"

	"github.com/fluxcd/pkg/oci"
)

// Default cache expiration time in seconds for ACR refresh token.
// TODO @souleb: This is copied from https://github.com/Azure/msi-acrpull/blob/0ca921a7740e561c7204d9c3b3b55c4e0b9bd7b9/pkg/authorizer/token_retriever.go#L21C2-L21C39
// as it is not provided by the Azure SDK. Check with the Azure SDK team to see if there is a better way to get this value.
const defaultCacheExpirationInSeconds = 600

// Client is an Azure ACR client which can log into the registry and return
// authorization information.
type Client struct {
	credential azcore.TokenCredential
	scheme     string
	proxyURL   *url.URL
}

// Option is a functional option for configuring the client.
type Option func(*Client)

// WithProxyURL sets the proxy URL for the client.
func WithProxyURL(proxyURL *url.URL) Option {
	return func(c *Client) {
		c.proxyURL = proxyURL
	}
}

// NewClient creates a new ACR client with default configurations.
func NewClient(opts ...Option) *Client {
	client := &Client{scheme: "https"}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// WithTokenCredential sets the token credential used by the ACR client.
func (c *Client) WithTokenCredential(tc azcore.TokenCredential) *Client {
	c.credential = tc
	return c
}

// WithScheme sets the scheme of the http request that the client makes.
func (c *Client) WithScheme(scheme string) *Client {
	c.scheme = scheme
	return c
}

// getLoginAuth returns authentication for ACR. The details needed for authentication
// are gotten from environment variable so there is no need to mount a host path.
// The endpoint is the registry server and will be queried for OAuth authorization token.
func (c *Client) getLoginAuth(ctx context.Context, registryURL string) (authn.AuthConfig, time.Time, error) {
	var authConfig authn.AuthConfig

	// Use default credentials if no token credential is provided.
	// NOTE: NewDefaultAzureCredential() performs a lot of environment lookup
	// for creating default token credential. Load it only when it's needed.
	if c.credential == nil {
		opts := &azidentity.DefaultAzureCredentialOptions{}
		if c.proxyURL != nil {
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.Proxy = http.ProxyURL(c.proxyURL)
			opts.Transport = &http.Client{Transport: transport}
		}

		cred, err := azidentity.NewDefaultAzureCredential(opts)
		if err != nil {
			return authConfig, time.Time{}, err
		}
		c.credential = cred
	}

	configurationEnvironment := getCloudConfiguration(registryURL)
	// Obtain access token using the token credential.
	armToken, err := c.credential.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{configurationEnvironment.Services[cloud.ResourceManager].Endpoint + "/" + ".default"},
	})
	if err != nil {
		return authConfig, time.Time{}, err
	}

	// Obtain ACR access token using exchanger.
	ex := newExchanger(registryURL, c.proxyURL)
	accessToken, err := ex.ExchangeACRAccessToken(string(armToken.Token))
	if err != nil {
		return authConfig, time.Time{}, fmt.Errorf("error exchanging token: %w", err)
	}

	expiresAt := time.Now().Add(defaultCacheExpirationInSeconds * time.Second)

	return authn.AuthConfig{
		// This is the acr username used by Azure
		// See documentation: https://docs.microsoft.com/en-us/azure/container-registry/container-registry-authentication?tabs=azure-cli#az-acr-login-with---expose-token
		Username: "00000000-0000-0000-0000-000000000000",
		Password: accessToken,
	}, expiresAt, nil
}

// getCloudConfiguration returns the cloud configuration based on the registry URL.
// List from https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/containers/azcontainerregistry/cloud_config.go#L16
func getCloudConfiguration(url string) cloud.Configuration {
	switch {
	case strings.HasSuffix(url, ".azurecr.cn"):
		return cloud.AzureChina
	case strings.HasSuffix(url, ".azurecr.us"):
		return cloud.AzureGovernment
	default:
		return cloud.AzurePublic
	}
}

// ValidHost returns if a given host is a Azure container registry.
// List from https://github.com/kubernetes/kubernetes/blob/v1.23.1/pkg/credentialprovider/azure/azure_credentials.go#L55
func ValidHost(host string) bool {
	for _, v := range []string{".azurecr.io", ".azurecr.cn", ".azurecr.de", ".azurecr.us"} {
		if strings.HasSuffix(host, v) {
			return true
		}
	}
	return false
}

// LoginWithExpiry attempts to get the authentication material for ACR.
// It returns the authentication material and the expiry time of the token.
// The caller can ensure that the passed image is a valid ACR image using ValidHost().
func (c *Client) LoginWithExpiry(ctx context.Context, autoLogin bool, image string, ref name.Reference) (authn.Authenticator, time.Time, error) {
	if autoLogin {
		logr.FromContextOrDiscard(ctx).Info("logging in to Azure ACR for " + image)
		// get registry host from image
		strArr := strings.SplitN(image, "/", 2)
		endpoint := fmt.Sprintf("%s://%s", c.scheme, strArr[0])
		authConfig, expiresAt, err := c.getLoginAuth(ctx, endpoint)
		if err != nil {
			logr.FromContextOrDiscard(ctx).Info("error logging into ACR " + err.Error())
			return nil, time.Time{}, err
		}

		auth := authn.FromConfig(authConfig)
		return auth, expiresAt, nil
	}
	return nil, time.Time{}, fmt.Errorf("ACR authentication failed: %w", oci.ErrUnconfiguredProvider)
}

// Login attempts to get the authentication material for ACR. The caller can
// ensure that the passed image is a valid ACR image using ValidHost().
func (c *Client) Login(ctx context.Context, autoLogin bool, image string, ref name.Reference) (authn.Authenticator, error) {
	auth, _, err := c.LoginWithExpiry(ctx, autoLogin, image, ref)
	return auth, err
}

// OIDCLogin attempts to get an Authenticator for the provided ACR registry URL endpoint.
//
// If you want to construct an Authenticator based on an image reference,
// you may want to use Login instead.
//
// Deprecated: Use LoginWithExpiry instead.
func (c *Client) OIDCLogin(ctx context.Context, registryUrl string) (authn.Authenticator, error) {
	authConfig, _, err := c.getLoginAuth(ctx, registryUrl)
	if err != nil {
		logr.FromContextOrDiscard(ctx).Info("error logging into ACR " + err.Error())
		return nil, err
	}

	auth := authn.FromConfig(authConfig)
	return auth, nil
}
