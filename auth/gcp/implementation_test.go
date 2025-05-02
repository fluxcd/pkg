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

package gcp_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	. "github.com/onsi/gomega"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google/externalaccount"
)

type mockImplementation struct {
	t *testing.T

	argConfig   externalaccount.Config
	argProxyURL *url.URL
}

func (m *mockImplementation) DefaultTokenSource(ctx context.Context, scope ...string) (oauth2.TokenSource, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(ctx).NotTo(BeNil())
	g.Expect(ctx.Value(oauth2.HTTPClient)).NotTo(BeNil())
	g.Expect(ctx.Value(oauth2.HTTPClient).(*http.Client)).NotTo(BeNil())
	g.Expect(ctx.Value(oauth2.HTTPClient).(*http.Client).Transport).NotTo(BeNil())
	g.Expect(ctx.Value(oauth2.HTTPClient).(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(ctx.Value(oauth2.HTTPClient).(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := ctx.Value(oauth2.HTTPClient).(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))
	g.Expect(scope).To(Equal([]string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email",
	}))
	return oauth2.StaticTokenSource(&oauth2.Token{}), nil
}

func (m *mockImplementation) NewTokenSource(ctx context.Context, conf externalaccount.Config) (oauth2.TokenSource, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(ctx).NotTo(BeNil())
	g.Expect(ctx.Value(oauth2.HTTPClient)).NotTo(BeNil())
	g.Expect(ctx.Value(oauth2.HTTPClient).(*http.Client)).NotTo(BeNil())
	g.Expect(ctx.Value(oauth2.HTTPClient).(*http.Client).Transport).NotTo(BeNil())
	g.Expect(ctx.Value(oauth2.HTTPClient).(*http.Client).Transport.(*http.Transport)).NotTo(BeNil())
	g.Expect(ctx.Value(oauth2.HTTPClient).(*http.Client).Transport.(*http.Transport).Proxy).NotTo(BeNil())
	proxyURL, err := ctx.Value(oauth2.HTTPClient).(*http.Client).Transport.(*http.Transport).Proxy(nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(proxyURL).To(Equal(m.argProxyURL))
	g.Expect(conf).To(Equal(m.argConfig))
	return oauth2.StaticTokenSource(&oauth2.Token{}), nil
}
