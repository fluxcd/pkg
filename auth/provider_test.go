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

package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/auth"
)

type mockProvider struct {
	t *testing.T

	returnName              string
	returnAudience          string
	returnIdentity          string
	returnIdentityErr       string
	returnRegistryErr       string
	returnRegistryInput     string
	returnRESTConfig        *auth.RESTConfig
	returnRESTConfigOptsErr string
	returnControllerToken   auth.Token
	returnAccessToken       auth.Token
	returnRegistryOptions   []auth.Option
	returnRegistryToken     *auth.ArtifactRegistryCredentials
	paramServiceAccount     corev1.ServiceAccount
	paramOIDCTokenClient    *http.Client
	paramArtifactRepository string
	paramCluster            string
	paramAccessToken        auth.Token
	paramAccessTokens       []auth.Token
	paramAllowShellOut      bool

	// For multi-token flow (RESTConfig)
	paramFirstScopes  []string
	paramSecondScopes []string
	firstCallMade     bool
	secondCallMade    bool
}

func (m *mockProvider) GetName() string {
	return m.returnName
}

func (m *mockProvider) NewControllerToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {
	m.checkOptions(opts...)
	return m.returnControllerToken, nil
}

func (m *mockProvider) GetAudience(ctx context.Context, serviceAccount corev1.ServiceAccount) (string, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(serviceAccount).To(Equal(m.paramServiceAccount))
	return m.returnAudience, nil
}

func (m *mockProvider) GetIdentity(serviceAccount corev1.ServiceAccount) (string, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(serviceAccount).To(Equal(m.paramServiceAccount))
	if m.returnIdentityErr != "" {
		return "", errors.New(m.returnIdentityErr)
	}
	return m.returnIdentity, nil
}

func (m *mockProvider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	serviceAccount corev1.ServiceAccount, opts ...auth.Option) (auth.Token, error) {

	m.t.Helper()
	g := NewWithT(m.t)

	// Verify the OIDC token.
	g.Expect(m.returnAudience).NotTo(BeEmpty())
	token, _, err := jwt.NewParser().ParseUnverified(oidcToken, jwt.MapClaims{})
	g.Expect(err).NotTo(HaveOccurred())
	iss, err := token.Claims.GetIssuer()
	g.Expect(err).NotTo(HaveOccurred())
	ctx = oidc.ClientContext(ctx, m.paramOIDCTokenClient)
	jwks := oidc.NewRemoteKeySet(ctx, iss+"openid/v1/jwks")
	_, err = oidc.NewVerifier(iss, jwks, &oidc.Config{
		ClientID:             m.returnAudience,
		SupportedSigningAlgs: []string{token.Method.Alg()},
	}).Verify(ctx, oidcToken)
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(serviceAccount).To(Equal(m.paramServiceAccount))

	m.checkOptions(opts...)

	return m.returnAccessToken, nil
}

func (m *mockProvider) ParseArtifactRepository(artifactRepository string) (string, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(artifactRepository).To(Equal(m.paramArtifactRepository))
	if m.returnRegistryErr != "" {
		return "", errors.New(m.returnRegistryErr)
	}
	return m.returnRegistryInput, nil
}

func (m *mockProvider) GetAccessTokenOptionsForArtifactRepository(artifactRepository string) ([]auth.Option, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(artifactRepository).To(Equal(m.paramArtifactRepository))
	return m.returnRegistryOptions, nil
}

func (m *mockProvider) NewArtifactRegistryCredentials(ctx context.Context, registryInput string,
	accessToken auth.Token, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(registryInput).To(Equal(m.paramArtifactRepository))
	g.Expect(accessToken).To(Equal(m.paramAccessToken))
	m.checkOptions(opts...)
	return m.returnRegistryToken, nil
}

func (m *mockProvider) GetAccessTokenOptionsForCluster(cluster string) ([][]auth.Option, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(cluster).To(Equal(m.paramCluster))
	if m.returnRESTConfigOptsErr != "" {
		return nil, errors.New(m.returnRESTConfigOptsErr)
	}
	return [][]auth.Option{{auth.WithScopes("first-token")}, {auth.WithScopes("second-token")}}, nil
}

func (m *mockProvider) NewRESTConfig(ctx context.Context, cluster string,
	accessTokens []auth.Token, opts ...auth.Option) (*auth.RESTConfig, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(cluster).To(Equal(m.paramCluster))
	g.Expect(accessTokens).To(Equal(m.paramAccessTokens))
	m.checkOptions(opts...)
	return m.returnRESTConfig, nil
}

func (m *mockProvider) checkOptions(opts ...auth.Option) {
	m.t.Helper()

	g := NewWithT(m.t)

	var o auth.Options
	o.Apply(opts...)

	// Determine which scopes to expect based on multi-token flow
	expectedScopes := []string{"scope1", "scope2"}
	if m.paramFirstScopes != nil && m.paramSecondScopes != nil {
		if !m.firstCallMade {
			expectedScopes = m.paramFirstScopes
			m.firstCallMade = true
		} else if !m.secondCallMade {
			expectedScopes = m.paramSecondScopes
			m.secondCallMade = true
		} else {
			expectedScopes = []string{"scope1", "scope2"}
		}
	}

	g.Expect(o.Scopes).To(Equal(expectedScopes))
	g.Expect(o.STSRegion).To(Equal("us-east-1"))
	g.Expect(o.STSEndpoint).To(Equal("https://sts.some-cloud.io"))
	g.Expect(o.ProxyURL).To(Equal(&url.URL{Scheme: "http", Host: "proxy.io:8080"}))
	g.Expect(o.AllowShellOut).To(Equal(m.paramAllowShellOut))
}
