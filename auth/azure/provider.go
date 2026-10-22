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
	"net/url"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-containerregistry/pkg/authn"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
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
func (p Provider) NewControllerToken(ctx context.Context, opts ...auth.Option) (auth.Credential, error) {

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

	return &AccessToken{token}, nil
}

// GetAudiences implements auth.Provider.
func (Provider) GetAudiences(_ context.Context, opts ...auth.Option) ([]string, error) {
	var o auth.Options
	o.Apply(opts...)
	if len(o.CredentialAudiences) > 0 {
		return o.CredentialAudiences, nil
	}
	return []string{cryptov1.DefaultAudienceAzure}, nil
}

// GetIdentity implements auth.Provider.
func (Provider) GetIdentity(opts ...auth.Option) (string, error) {
	var o auth.Options
	o.Apply(opts...)
	return fmt.Sprintf("%s/%s", o.AzureTenantID, o.AzureClientID), nil
}

// NewAccessCredentialForIdentity implements auth.Provider.
func (p Provider) NewAccessCredentialForIdentity(ctx context.Context, identity auth.Credential,
	opts ...auth.Option) (auth.Credential, error) {

	var o auth.Options
	o.Apply(opts...)

	jwtCredential, ok := identity.(*auth.JWTCredential)
	if !ok {
		return nil, fmt.Errorf("expected *auth.JWTCredential, got %T", identity)
	}

	tenantID := o.AzureTenantID
	clientID := o.AzureClientID

	azOpts := &azidentity.ClientAssertionCredentialOptions{
		ClientOptions: azcore.ClientOptions{
			Transport: o.GetHTTPClient(),
		},
	}

	cred, err := p.impl().NewClientAssertionCredential(tenantID, clientID, func(context.Context) (string, error) {
		return jwtCredential.Token, nil
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

	return &AccessToken{token}, nil
}

// GetAccessCredentialOptionsForArtifactRepository implements auth.Provider.
func (p Provider) GetAccessCredentialOptionsForArtifactRepository(artifactRepository string,
	_ ...auth.Option) ([]auth.Option, error) {
	// Azure requires scopes for getting access credentials. Here we compute
	// the scope for ACR, which is based on the registry host.

	registry, err := cryptov1.GetRegistryFromArtifactRepository(artifactRepository)
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

// ParseArtifactRepository implements auth.Provider.
// ParseArtifactRepository returns the ACR registry host.
func (Provider) ParseArtifactRepository(artifactRepository string) (string, error) {
	registry, err := cryptov1.ParseArtifactRepository(ProviderName, artifactRepository)
	if err == nil {
		return registry, nil
	}

	// Fall back to the container registry suffix configured in the Azure
	// environment file (AZURE_ENVIRONMENT_FILEPATH), which supports sovereign
	// clouds with a custom suffix.
	if !hasEnvironmentFile() {
		return "", err
	}
	host, err := cryptov1.GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		return "", err
	}
	registrySuffix, err := getContainerRegistryDNSSuffix()
	if err != nil {
		return "", fmt.Errorf("failed to get container registry suffix from environment file: %w", err)
	}
	if strings.HasSuffix(host, registrySuffix) {
		return host, nil
	}
	return "", fmt.Errorf("invalid Azure registry: '%s'. must end with %s", host, registrySuffix)
}

// NewArtifactRegistryCredentials implements auth.Provider.
func (p Provider) NewArtifactRegistryCredentials(ctx context.Context, registry string,
	accessCredential auth.Credential, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {

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

	// Exchange the access credential for an ACR token.
	grantType := azcontainerregistry.PostContentSchemaGrantTypeAccessToken
	service := registry
	tokenOpts := &azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenOptions{
		AccessToken: &accessCredential.(*AccessToken).Token,
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

// GetAccessCredentialOptionsForCluster implements auth.Provider.
func (Provider) GetAccessCredentialOptionsForCluster(opts ...auth.Option) ([][]auth.Option, error) {
	var o auth.Options
	o.Apply(opts...)

	var acOpts [][]auth.Option

	// Token used for impersonating the Managed Identity inside the AKS cluster.
	const aksScope = "6dae42f8-4368-4678-94ff-3960e28e3630/.default"
	aksTokenOpts := []auth.Option{auth.WithScopes(aksScope)}
	acOpts = append(acOpts, aksTokenOpts)

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
		acOpts = append(acOpts, armTokenOpts)
	}

	return acOpts, nil
}

// NewRESTConfig implements auth.Provider.
func (p Provider) NewRESTConfig(ctx context.Context, accessCredentials []auth.Credential,
	opts ...auth.Option) (*auth.RESTConfig, error) {

	aksToken := accessCredentials[0].(*AccessToken)

	var armToken *AccessToken
	if len(accessCredentials) == 2 {
		armToken = accessCredentials[1].(*AccessToken)
	}

	var o auth.Options
	o.Apply(opts...)

	// Describe the cluster resource to get missing CA or endpoint.
	host := o.ClusterAddress
	caData := []byte(o.CAData)
	if host == "" || len(caData) == 0 {
		subscriptionID := o.AzureSubscriptionID
		resourceGroup := o.AzureResourceGroup
		clusterName := o.AzureClusterName
		if subscriptionID == "" || resourceGroup == "" || clusterName == "" {
			return nil, fmt.Errorf("Azure subscription ID, resource group and cluster name are required")
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
				clusterName)
		}

		// Parse specified cluster address.
		var canonicalHost string
		if host != "" {
			var err error
			canonicalHost, err = cryptov1.ParseClusterAddress(host)
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
			canonicalHostFromAPI, err := cryptov1.ParseClusterAddress(conf.Host)
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
				return nil, fmt.Errorf("no kubeconfig found for AKS cluster %s", clusterName)
			}
			return nil, fmt.Errorf("no kubeconfig found for AKS cluster %s matching the specified address '%s'. cluster addresses: [%s]",
				clusterName, o.ClusterAddress, strings.Join(addresses, ", "))
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

// GetAccessCredentialOptionsForGitRepository implements auth.GitCredentialsProvider.
// Azure DevOps requires the DevOps scope for obtaining an access credential that
// can be used as a bearer token against Azure Repos.
func (Provider) GetAccessCredentialOptionsForGitRepository(*url.URL) ([]auth.Option, error) {
	return []auth.Option{auth.WithScopes(ScopeDevOps)}, nil
}

// ParseGitRepository implements auth.GitCredentialsProvider.
// Azure DevOps access credentials are not bound to the Git repository URL,
// so this returns a constant input that does not affect the cache key.
func (Provider) ParseGitRepository(*url.URL) (string, error) {
	return "azure-devops", nil
}

// NewGitCredentials implements auth.GitCredentialsProvider.
func (Provider) NewGitCredentials(_ context.Context, _ string,
	accessCredential auth.Credential, _ ...auth.Option) (*auth.GitCredentials, error) {

	credential, ok := accessCredential.(*AccessToken)
	if !ok {
		return nil, fmt.Errorf("failed to cast credential to Azure access token: %T", accessCredential)
	}
	return &auth.GitCredentials{
		BearerToken: credential.Token,
		ExpiresAt:   credential.ExpiresOn,
	}, nil
}

func (p Provider) impl() Implementation {
	if p.Implementation == nil {
		return implementation{}
	}
	return p.Implementation
}
