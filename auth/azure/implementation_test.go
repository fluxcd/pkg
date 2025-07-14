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

package azure_test

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/auth/azure"
)

type mockImplementation struct {
	t *testing.T

	shellOut bool

	expectAKSAPICall bool

	argTenantID      string
	argClientID      string
	argOIDCToken     string
	argProxyURL      *url.URL
	argScopes        []string
	argToken         string
	argRegistry      string
	argSubscription  string
	argResourceGroup string
	argClusterName   string

	// For dual-token flow (RESTConfig)
	argFirstScopes  []string
	argSecondScopes []string
	firstCallMade   bool

	returnToken       string
	returnACRToken    string
	returnCluster     armcontainerservice.ManagedCluster
	returnKubeconfigs []*armcontainerservice.CredentialResult
}

type mockTokenCredential struct {
	t *testing.T

	argScopes []string

	returnToken string
}

type mockAKSClient struct {
	t *testing.T

	argResourceGroup string
	argClusterName   string

	returnCluster     armcontainerservice.ManagedCluster
	returnKubeconfigs []*armcontainerservice.CredentialResult
}

func (m *mockImplementation) NewDefaultAzureCredential(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(m.shellOut).To(BeTrue())
	return m.newDefaultAzureCredential(options)
}

func (m *mockImplementation) NewDefaultAzureCredentialWithoutShellOut(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(m.shellOut).To(BeFalse())
	return m.newDefaultAzureCredential(options)
}

func (m *mockImplementation) newDefaultAzureCredential(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(options.Transport).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client)).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client).Transport).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := options.Transport.(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))

	// Determine which scopes to expect based on dual-token flow
	expectedScopes := m.argScopes
	if m.argFirstScopes != nil && m.argSecondScopes != nil {
		if !m.firstCallMade {
			expectedScopes = m.argFirstScopes
			m.firstCallMade = true
		} else {
			expectedScopes = m.argSecondScopes
		}
	}

	return &mockTokenCredential{t: m.t, argScopes: expectedScopes, returnToken: m.returnToken}, nil
}

func (m *mockImplementation) NewClientAssertionCredential(tenantID string, clientID string, getAssertion func(context.Context) (string, error), options *azidentity.ClientAssertionCredentialOptions) (azcore.TokenCredential, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(tenantID).To(Equal(m.argTenantID))
	g.Expect(clientID).To(Equal(m.argClientID))
	g.Expect(getAssertion).NotTo(BeNil())
	oidcToken, err := getAssertion(context.Background())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(oidcToken).To(Equal(m.argOIDCToken))
	g.Expect(options).NotTo(BeNil())
	g.Expect(options.Transport).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client)).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client).Transport).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := options.Transport.(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))

	// Determine which scopes to expect based on dual-token flow
	expectedScopes := m.argScopes
	if m.argFirstScopes != nil && m.argSecondScopes != nil {
		if !m.firstCallMade {
			expectedScopes = m.argFirstScopes
			m.firstCallMade = true
		} else {
			expectedScopes = m.argSecondScopes
		}
	}

	return &mockTokenCredential{t: m.t, argScopes: expectedScopes, returnToken: m.returnToken}, nil
}

func (m *mockImplementation) ExchangeAADAccessTokenForACRRefreshToken(ctx context.Context, client *azcontainerregistry.AuthenticationClient, grantType azcontainerregistry.PostContentSchemaGrantType, service string, options *azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenOptions) (azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenResponse, error) {
	m.t.Helper()
	g := NewWithT(m.t)

	// Assert registry endpoint.
	endpointField := reflect.ValueOf(client).Elem().FieldByName("endpoint")
	endpointValue := reflect.NewAt(endpointField.Type(), unsafe.Pointer(endpointField.UnsafeAddr())).Elem().Interface().(string)
	g.Expect(endpointValue).To(Equal("https://" + m.argRegistry))

	// Assert proxy URL.
	azcoreClientField := reflect.ValueOf(client).Elem().FieldByName("internal")
	azcoreClientValue := reflect.NewAt(azcoreClientField.Type(), unsafe.Pointer(azcoreClientField.UnsafeAddr())).Elem().Interface().(*azcore.Client)
	g.Expect(azcoreClientValue).NotTo(BeNil())
	pipeline := azcoreClientValue.Pipeline()
	g.Expect(pipeline).NotTo(BeNil())
	pipelineValue := reflect.ValueOf(pipeline)
	pipelinePtr := reflect.New(pipelineValue.Type())
	pipelinePtr.Elem().Set(pipelineValue)
	policiesField := pipelinePtr.Elem().FieldByName("policies")
	policiesValue := reflect.NewAt(policiesField.Type(), unsafe.Pointer(policiesField.UnsafeAddr())).Elem().Interface().([]policy.Policy)
	g.Expect(policiesValue).NotTo(BeNil())
	transportPolicy := policiesValue[len(policiesValue)-1]
	transportPolicyValue := reflect.ValueOf(transportPolicy)
	transportPolicyPtr := reflect.New(transportPolicyValue.Type())
	transportPolicyPtr.Elem().Set(transportPolicyValue)
	transportField := transportPolicyPtr.Elem().FieldByName("trans")
	transportValue := reflect.NewAt(transportField.Type(), unsafe.Pointer(transportField.UnsafeAddr())).Elem().Interface().(policy.Transporter)
	g.Expect(transportValue).NotTo(BeNil())
	g.Expect(transportValue.(*http.Client)).NotTo(BeNil())
	g.Expect(transportValue.(*http.Client).Transport).NotTo(BeNil())
	g.Expect(transportValue.(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(transportValue.(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := transportValue.(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))

	// Assert trivial inputs.
	g.Expect(grantType).To(Equal(azcontainerregistry.PostContentSchemaGrantTypeAccessToken))
	g.Expect(service).To(Equal(m.argRegistry))
	g.Expect(options).To(Equal(&azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenOptions{
		AccessToken: &m.argToken,
	}))

	return azcontainerregistry.AuthenticationClientExchangeAADAccessTokenForACRRefreshTokenResponse{
		ACRRefreshToken: azcontainerregistry.ACRRefreshToken{RefreshToken: &m.returnACRToken},
	}, nil
}

func (m *mockImplementation) NewManagedClustersClient(subscriptionID string, credential azcore.TokenCredential, options *arm.ClientOptions) (azure.AKSClient, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(m.expectAKSAPICall).To(BeTrue())
	g.Expect(subscriptionID).To(Equal(m.argSubscription))
	g.Expect(credential).NotTo(BeNil())
	token, err := credential.GetToken(context.Background(), policy.TokenRequestOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token.Token).To(Equal(m.argToken))
	g.Expect(options).NotTo(BeNil())
	g.Expect(options.Transport).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client)).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client).Transport).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(options.Transport.(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := options.Transport.(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))
	return &mockAKSClient{
		t:                 m.t,
		argResourceGroup:  m.argResourceGroup,
		argClusterName:    m.argClusterName,
		returnCluster:     m.returnCluster,
		returnKubeconfigs: m.returnKubeconfigs,
	}, nil
}

func (m *mockAKSClient) Get(ctx context.Context, resourceGroupName string, resourceName string, options *armcontainerservice.ManagedClustersClientGetOptions) (armcontainerservice.ManagedClustersClientGetResponse, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(ctx).NotTo(BeNil())
	g.Expect(resourceGroupName).To(Equal(m.argResourceGroup))
	g.Expect(resourceName).To(Equal(m.argClusterName))
	g.Expect(options).To(BeNil())
	return armcontainerservice.ManagedClustersClientGetResponse{
		ManagedCluster: m.returnCluster,
	}, nil
}

func (m *mockAKSClient) ListClusterUserCredentials(ctx context.Context, resourceGroupName string, resourceName string, options *armcontainerservice.ManagedClustersClientListClusterUserCredentialsOptions) (armcontainerservice.ManagedClustersClientListClusterUserCredentialsResponse, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(ctx).NotTo(BeNil())
	g.Expect(resourceGroupName).To(Equal(m.argResourceGroup))
	g.Expect(resourceName).To(Equal(m.argClusterName))
	g.Expect(options).To(BeNil())
	return armcontainerservice.ManagedClustersClientListClusterUserCredentialsResponse{
		CredentialResults: armcontainerservice.CredentialResults{
			Kubeconfigs: m.returnKubeconfigs,
		},
	}, nil
}

func (m *mockTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(options.Scopes).To(Equal(m.argScopes))
	return azcore.AccessToken{
		Token:     m.returnToken,
		ExpiresOn: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), // Fixed expiry for testing
	}, nil
}
