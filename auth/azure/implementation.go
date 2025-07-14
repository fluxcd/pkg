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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
)

// Implementation provides the required methods of the Azure libraries.
type Implementation interface {
	NewDefaultAzureCredential(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error)
	NewDefaultAzureCredentialWithoutShellOut(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error)
	NewClientAssertionCredential(tenantID string, clientID string, getAssertion func(context.Context) (string, error), options *azidentity.ClientAssertionCredentialOptions) (azcore.TokenCredential, error)
	ExchangeAADAccessTokenForACRRefreshToken(ctx context.Context, client *azcontainerregistry.AuthenticationClient, grantType azcontainerregistry.PostContentSchemaGrantType, service string, options *azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenOptions) (azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenResponse, error)
	NewManagedClustersClient(subscriptionID string, credential azcore.TokenCredential, options *arm.ClientOptions) (AKSClient, error)
}

// AKSClient provides the required methods of the AKS client.
type AKSClient interface {
	Get(ctx context.Context, resourceGroupName string, resourceName string, options *armcontainerservice.ManagedClustersClientGetOptions) (armcontainerservice.ManagedClustersClientGetResponse, error)
	ListClusterUserCredentials(ctx context.Context, resourceGroupName string, resourceName string, options *armcontainerservice.ManagedClustersClientListClusterUserCredentialsOptions) (armcontainerservice.ManagedClustersClientListClusterUserCredentialsResponse, error)
}

type implementation struct{}

func (implementation) NewDefaultAzureCredential(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
	return azidentity.NewDefaultAzureCredential(options)
}

func (implementation) NewDefaultAzureCredentialWithoutShellOut(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
	return newDefaultAzureCredential(options)
}

func (implementation) NewClientAssertionCredential(tenantID string, clientID string, getAssertion func(context.Context) (string, error), options *azidentity.ClientAssertionCredentialOptions) (azcore.TokenCredential, error) {
	return azidentity.NewClientAssertionCredential(tenantID, clientID, getAssertion, options)
}

func (implementation) ExchangeAADAccessTokenForACRRefreshToken(ctx context.Context, client *azcontainerregistry.AuthenticationClient, grantType azcontainerregistry.PostContentSchemaGrantType, service string, options *azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenOptions) (azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenResponse, error) {
	return client.ExchangeAADAccessTokenForACRRefreshToken(ctx, grantType, service, options)
}

func (implementation) NewManagedClustersClient(subscriptionID string, credential azcore.TokenCredential, options *arm.ClientOptions) (AKSClient, error) {
	return armcontainerservice.NewManagedClustersClient(subscriptionID, credential, options)
}
