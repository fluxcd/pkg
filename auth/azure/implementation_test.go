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
	"unsafe"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
	. "github.com/onsi/gomega"
)

type mockImplementation struct {
	t *testing.T

	argTenantID  string
	argClientID  string
	argOIDCToken string
	argProxyURL  *url.URL
	argScopes    []string
	argToken     string
	argRegistry  string

	returnToken    string
	returnACRToken string
}

type mockTokenCredential struct {
	t *testing.T

	argScopes []string

	returnToken string
}

func (m *mockImplementation) NewDefaultAzureCredential(options azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
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
	return &mockTokenCredential{t: m.t, argScopes: m.argScopes, returnToken: m.returnToken}, nil
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
	return &mockTokenCredential{t: m.t, argScopes: m.argScopes, returnToken: m.returnToken}, nil
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

func (m *mockTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(options.Scopes).To(Equal(m.argScopes))
	return azcore.AccessToken{Token: m.returnToken}, nil
}
