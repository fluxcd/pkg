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
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	. "github.com/onsi/gomega"
)

type mockImplementation struct {
	t *testing.T

	argTenantID  string
	argClientID  string
	argOIDCToken string
	argProxyURL  *url.URL
	argScopes    []string

	returnResp *http.Response
}

type mockTokenCredential struct {
	t *testing.T

	argScopes []string
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
	return &mockTokenCredential{t: m.t, argScopes: m.argScopes}, nil
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
	return &mockTokenCredential{t: m.t, argScopes: m.argScopes}, nil
}

func (m *mockImplementation) SendRequest(req *http.Request, client *http.Client) (*http.Response, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(client).NotTo(BeNil())
	g.Expect(client.Transport).NotTo(BeNil())
	g.Expect(client.Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(client.Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := client.Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))
	return m.returnResp, nil
}

func (m *mockTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(options.Scopes).To(Equal(m.argScopes))
	return azcore.AccessToken{}, nil
}
