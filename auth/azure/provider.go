/*
Copyright 2025 The Flux authors

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
	"regexp"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-containerregistry/pkg/authn"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/fluxcd/pkg/auth"
)

// ProviderName is the name of the Azure authentication provider.
const ProviderName = "azure"

// Provider implements the auth.Provider interface for Azure authentication.
type Provider struct{ Implementation }

// GetName implements auth.Provider.
func (Provider) GetName() string {
	return ProviderName
}

// NewControllerToken implements auth.Provider.
func (p Provider) NewControllerToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	var azOpts azidentity.DefaultAzureCredentialOptions

	if hc := o.GetHTTPClient(); hc != nil {
		azOpts.Transport = hc
	}

	cred, err := p.impl().NewDefaultAzureCredential(azOpts)
	if err != nil {
		return nil, err
	}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: o.Scopes,
	})
	if err != nil {
		return nil, err
	}

	return &Token{token}, nil
}

// GetAudience implements auth.Provider.
func (Provider) GetAudience(context.Context, corev1.ServiceAccount) (string, error) {
	return "api://AzureADTokenExchange", nil
}

// GetIdentity implements auth.Provider.
func (Provider) GetIdentity(serviceAccount corev1.ServiceAccount) (string, error) {
	return getIdentity(serviceAccount)
}

// NewTokenForServiceAccount implements auth.Provider.
func (p Provider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	serviceAccount corev1.ServiceAccount, opts ...auth.Option) (auth.Token, error) {

	var o auth.Options
	o.Apply(opts...)

	identity, err := getIdentity(serviceAccount)
	if err != nil {
		return nil, err
	}
	s := strings.Split(identity, "/")
	tenantID, clientID := s[0], s[1]

	azOpts := &azidentity.ClientAssertionCredentialOptions{}

	if hc := o.GetHTTPClient(); hc != nil {
		azOpts.Transport = hc
	}

	cred, err := p.impl().NewClientAssertionCredential(tenantID, clientID, func(context.Context) (string, error) {
		return oidcToken, nil
	}, azOpts)
	if err != nil {
		return nil, err
	}
	token, err := cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: o.Scopes,
	})
	if err != nil {
		return nil, err
	}

	return &Token{token}, nil
}

// GetAccessTokenOptionsForArtifactRepository implements auth.Provider.
func (p Provider) GetAccessTokenOptionsForArtifactRepository(artifactRepository string) ([]auth.Option, error) {
	// Azure requires scopes for getting access tokens. Here we compute
	// the scope for ACR, which is based on the registry host.

	registry, err := auth.GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		return nil, err
	}

	var conf *cloud.Configuration
	switch {
	case strings.HasSuffix(registry, ".azurecr.cn"):
		conf = &cloud.AzureChina
	case strings.HasSuffix(registry, ".azurecr.us"):
		conf = &cloud.AzureGovernment
	default:
		conf = &cloud.AzurePublic
	}
	acrScope := conf.Services[cloud.ResourceManager].Endpoint + "/.default"

	return []auth.Option{auth.WithScopes(acrScope)}, nil
}

// https://github.com/kubernetes/kubernetes/blob/v1.23.1/pkg/credentialprovider/azure/azure_credentials.go#L55
const registryPattern = `^.+\.(azurecr\.io|azurecr\.cn|azurecr\.de|azurecr\.us)$`

var registryRegex = regexp.MustCompile(registryPattern)

// ParseArtifactRepository implements auth.Provider.
// ParseArtifactRepository returns the ACR registry host.
func (Provider) ParseArtifactRepository(artifactRepository string) (string, error) {
	registry, err := auth.GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		return "", err
	}

	if !registryRegex.MatchString(registry) {
		return "", fmt.Errorf("invalid Azure registry: '%s'. must match %s",
			registry, registryPattern)
	}

	// For issuing Azure registry credentials the registry host is required.
	return registry, nil
}

// NewArtifactRegistryCredentials implements auth.Provider.
func (p Provider) NewArtifactRegistryCredentials(ctx context.Context, registry string,
	accessToken auth.Token, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {

	var o auth.Options
	o.Apply(opts...)

	// Create the ACR authentication client.
	endpoint := fmt.Sprintf("https://%s", registry)
	var clientOpts azcontainerregistry.AuthenticationClientOptions
	if hc := o.GetHTTPClient(); hc != nil {
		clientOpts.Transport = hc
	}
	client, err := azcontainerregistry.NewAuthenticationClient(endpoint, &clientOpts)
	if err != nil {
		return nil, err
	}

	// Exchange the access token for an ACR token.
	grantType := azcontainerregistry.PostContentSchemaGrantTypeAccessToken
	service := registry
	tokenOpts := &azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenOptions{
		AccessToken: &accessToken.(*Token).Token,
	}
	resp, err := p.impl().ExchangeAADAccessTokenForACRRefreshToken(ctx, client, grantType, service, tokenOpts)
	if err != nil {
		return nil, err
	}
	token := *resp.RefreshToken

	// Parse the refresh token to get the expiry time.
	var claims jwt.MapClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil {
		return nil, err
	}
	expiry, err := claims.GetExpirationTime()
	if err != nil {
		return nil, err
	}

	// Return the credentials.
	return &auth.ArtifactRegistryCredentials{
		Authenticator: authn.FromConfig(authn.AuthConfig{
			// https://docs.microsoft.com/en-us/azure/container-registry/container-registry-authentication?tabs=azure-cli#az-acr-login-with---expose-token
			Username: "00000000-0000-0000-0000-000000000000",
			Password: token,
		}),
		ExpiresAt: expiry.Time,
	}, nil
}

// NewRESTConfig implements auth.Provider.
func (p Provider) NewRESTConfig(ctx context.Context, cluster, canonicalAddress string,
	opts ...auth.Option) (*auth.RESTConfig, error) {

	subscriptionID, resourceGroup, clusterName, err := parseCluster(cluster)
	if err != nil {
		return nil, err
	}

	var o auth.Options
	o.Apply(opts...)
	var restConfigTransport http.RoundTripper
	if hc := o.GetHTTPClient(); hc != nil {
		restConfigTransport = hc.Transport
	}

	// Create client for describing the cluster resource.
	armScope := cloud.AzurePublic.Services[cloud.ResourceManager].Audience + "/.default"
	armToken, err := auth.GetAccessToken(ctx, p, append(opts, auth.WithScopes(armScope))...)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token for describing AKS cluster: %w", err)
	}
	var clientOpts arm.ClientOptions
	if hc := o.GetHTTPClient(); hc != nil {
		clientOpts.Transport = hc
	}
	client, err := armcontainerservice.NewManagedClustersClient(subscriptionID,
		armToken.(*Token).credential(), &clientOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for describing AKS cluster: %w", err)
	}

	// Describe the cluster resource.
	clusterResource, err := client.Get(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to describe AKS cluster: %w", err)
	}

	// List kubeconfigs for this AKS cluster. We need to find the one
	// matching the canonical address.
	resp, err := client.ListClusterUserCredentials(ctx, resourceGroup, clusterName, nil)
	if err != nil {
		return nil, err
	}
	var restConfig *rest.Config
	for i, kc := range resp.Kubeconfigs {
		kubeconfig, err := clientcmd.NewClientConfigFromBytes(kc.Value)
		if err != nil {
			return nil, fmt.Errorf("failed to parse kubeconfig[%d]: %w", i, err)
		}
		restConfig, err = kubeconfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create restconfig from kubeconfig[%d]: %w", i, err)
		}
		confAddress, err := auth.ParseClusterAddress(restConfig.Host)
		if err != nil {
			return nil, fmt.Errorf("failed to parse kubeconfig[%d] address '%s': %w", i, restConfig.Host, err)
		}
		if confAddress == canonicalAddress {
			break
		}
	}
	if restConfig == nil {
		return nil, fmt.Errorf("no kubeconfig found for AKS cluster %s with address %s", cluster, canonicalAddress)
	}

	// If the cluster is not integrated with AAD or Azure RBAC is not enabled,
	// return the static mTLS-based REST config.
	if p := clusterResource.Properties.AADProfile; p == nil || !*p.EnableAzureRBAC {
		restConfig.Transport = restConfigTransport
		return &auth.RESTConfig{
			Config:    restConfig,
			ExpiresAt: time.Now().Add(time.Hour),
		}, nil
	}

	// Both AAD and Azure RBAC are enabled. Let's impersonate the
	// managed identity inside the cluster.
	const aksScope = "6dae42f8-4368-4678-94ff-3960e28e3630/.default"
	aksToken, err := auth.GetAccessToken(ctx, p, append(opts, auth.WithScopes(aksScope))...)
	if err != nil {
		return nil, fmt.Errorf("failed to get access token for AKS cluster: %w", err)
	}
	return &auth.RESTConfig{
		Config: &rest.Config{
			Host:        restConfig.Host,
			BearerToken: aksToken.(*Token).Token,
			Transport:   restConfigTransport,
			TLSClientConfig: rest.TLSClientConfig{
				CAData: restConfig.TLSClientConfig.CAData,
			},
		},
		ExpiresAt: aksToken.(*Token).ExpiresOn,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}
