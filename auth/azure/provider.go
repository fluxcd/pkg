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
	"os"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
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

	azOpts := azidentity.DefaultAzureCredentialOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: o.GetHTTPClient(),
		},
	}

	credFunc := p.impl().NewDefaultAzureCredentialWithoutShellOut
	if o.AllowShellOut {
		credFunc = p.impl().NewDefaultAzureCredential
	}
	cred, err := credFunc(&azOpts)
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

// GetAudiences implements auth.Provider.
func (Provider) GetAudiences(context.Context, corev1.ServiceAccount) ([]string, error) {
	return []string{"api://AzureADTokenExchange"}, nil
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

	azOpts := &azidentity.ClientAssertionCredentialOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: o.GetHTTPClient(),
		},
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
	case hasEnvironmentFile():
		var err error
		conf, err = getCloudConfigFromEnvironment()
		if err != nil {
			return nil, err
		}
	case strings.HasSuffix(registry, ".azurecr.cn"):
		conf = &cloud.AzureChina
	case strings.HasSuffix(registry, ".azurecr.us"):
		conf = &cloud.AzureGovernment
	default:
		conf = &cloud.AzurePublic
	}

	var acrScope string
	if acrService, ok := conf.Services[azcontainerregistry.ServiceName]; ok {
		acrScope = acrService.Audience + "/.default"
	} else {
		// Fallback for custom environments that don't define ACR service config.
		acrScope = conf.Services[cloud.ResourceManager].Endpoint + "/.default"
	}

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

	// For issuing Azure registry credentials the registry host is required.
	if registryRegex.MatchString(registry) {
		return registry, nil
	}

	// Check if environment variable is configured for container registry suffix
	if hasEnvironmentFile() {
		// Load the environment configuration from the file
		registrySuffix, err := getContainerRegistryDNSSuffix()
		if err != nil {
			return "", fmt.Errorf("failed to get container registry suffix from environment file: %w", err)
		}
		if strings.HasSuffix(registry, registrySuffix) {
			return registry, nil
		}
		return "", fmt.Errorf("invalid Azure registry: '%s'. must end with %s",
			registry, registrySuffix)
	}

	return "", fmt.Errorf("invalid Azure registry: '%s'. must match %s",
		registry, registryPattern)
}

// NewArtifactRegistryCredentials implements auth.Provider.
func (p Provider) NewArtifactRegistryCredentials(ctx context.Context, registry string,
	accessToken auth.Token, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {

	var o auth.Options
	o.Apply(opts...)

	// Create the ACR authentication client.
	endpoint := fmt.Sprintf("https://%s", registry)
	clientOpts := azcontainerregistry.AuthenticationClientOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: o.GetHTTPClient(),
		},
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

// GetAccessTokenOptionsForCluster implements auth.Provider.
func (Provider) GetAccessTokenOptionsForCluster(opts ...auth.Option) ([][]auth.Option, error) {
	var o auth.Options
	o.Apply(opts...)

	var atOpts [][]auth.Option

	// Token used for impersonating the Managed Identity inside the AKS cluster.
	const aksScope = "6dae42f8-4368-4678-94ff-3960e28e3630/.default"
	aksTokenOpts := []auth.Option{auth.WithScopes(aksScope)}
	atOpts = append(atOpts, aksTokenOpts)

	// Token needed for looking up details of the cluster resource.
	if o.ClusterAddress == "" || o.CAData == "" {
		conf := &cloud.AzurePublic
		switch authorityHost := os.Getenv("AZURE_AUTHORITY_HOST"); {
		case hasEnvironmentFile():
			var err error
			conf, err = getCloudConfigFromEnvironment()
			if err != nil {
				return nil, err
			}
		case strings.Contains(authorityHost, "chinacloudapi.cn"):
			conf = &cloud.AzureChina
		case strings.Contains(authorityHost, "microsoftonline.us"):
			conf = &cloud.AzureGovernment
		}
		armScope := conf.Services[cloud.ResourceManager].Audience + "/.default"
		armTokenOpts := []auth.Option{auth.WithScopes(armScope)}
		atOpts = append(atOpts, armTokenOpts)
	}

	return atOpts, nil
}

// NewRESTConfig implements auth.Provider.
func (p Provider) NewRESTConfig(ctx context.Context, accessTokens []auth.Token,
	opts ...auth.Option) (*auth.RESTConfig, error) {

	aksToken := accessTokens[0].(*Token)

	var armToken *Token
	if len(accessTokens) == 2 {
		armToken = accessTokens[1].(*Token)
	}

	var o auth.Options
	o.Apply(opts...)

	// Describe the cluster resource to get missing CA or endpoint.
	host := o.ClusterAddress
	caData := []byte(o.CAData)
	if host == "" || len(caData) == 0 {
		cluster := o.ClusterResource
		subscriptionID, resourceGroup, clusterName, err := parseCluster(cluster)
		if err != nil {
			return nil, err
		}

		// Create client for describing the cluster resource.
		clientOpts := arm.ClientOptions{
			ClientOptions: azcore.ClientOptions{
				Transport: o.GetHTTPClient(),
			},
		}
		client, err := p.impl().NewManagedClustersClient(
			subscriptionID, armToken.credential(), &clientOpts)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for describing AKS cluster: %w", err)
		}

		// Describe the cluster resource.
		clusterResource, err := client.Get(ctx, resourceGroup, clusterName, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to describe AKS cluster: %w", err)
		}

		// We only support clusters with Microsoft Entra ID integration enabled.
		if clusterResource.Properties.AADProfile == nil {
			return nil, fmt.Errorf("AKS cluster %s does not have Microsoft Entra ID integration enabled. "+
				"See docs for enabling: https://learn.microsoft.com/en-us/azure/aks/enable-authentication-microsoft-entra-id",
				cluster)
		}

		// Parse specified cluster address.
		var canonicalHost string
		if host != "" {
			var err error
			canonicalHost, err = auth.ParseClusterAddress(host)
			if err != nil {
				return nil, fmt.Errorf("failed to parse specified cluster address '%s': %w", host, err)
			}
		}

		// List kubeconfigs for this AKS cluster. We need to find the one
		// matching the canonical address, or the first one if no address
		// is specified.
		resp, err := client.ListClusterUserCredentials(ctx, resourceGroup, clusterName, nil)
		if err != nil {
			return nil, err
		}
		var restConfig *rest.Config
		var addresses []string
		for i, kc := range resp.Kubeconfigs {
			conf, err := clientcmd.RESTConfigFromKubeConfig(kc.Value)
			if err != nil {
				return nil, fmt.Errorf("failed to parse kubeconfig[%d]: %w", i, err)
			}
			addresses = append(addresses, fmt.Sprintf("'%s'", conf.Host))
			canonicalHostFromAPI, err := auth.ParseClusterAddress(conf.Host)
			if err != nil {
				return nil, fmt.Errorf("failed to parse address '%s' from kubeconfig[%d]: %w", conf.Host, i, err)
			}
			if canonicalHost == "" || canonicalHostFromAPI == canonicalHost {
				restConfig = conf
				break
			}
		}
		if restConfig == nil {
			if canonicalHost == "" {
				return nil, fmt.Errorf("no kubeconfig found for AKS cluster %s", cluster)
			}
			return nil, fmt.Errorf("no kubeconfig found for AKS cluster %s matching the specified address '%s'. cluster addresses: [%s]",
				cluster, o.ClusterAddress, strings.Join(addresses, ", "))
		}

		// Update host and CA with cluster details.
		if host == "" {
			host = restConfig.Host
		}
		if len(caData) == 0 {
			caData = restConfig.CAData
		}
	}

	// Build and return the REST config.
	return &auth.RESTConfig{
		Host:        host,
		BearerToken: aksToken.Token,
		CAData:      caData,
		ExpiresAt:   aksToken.ExpiresOn,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}
