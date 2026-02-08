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

// mockIdentity implements auth.Identity for testing.
type mockIdentity string

func (m mockIdentity) String() string { return string(m) }

// mockImpersonationIdentity can be JSON-unmarshaled from impersonation annotations.
type mockImpersonationIdentity struct {
	RoleArn string `json:"roleArn"`
}

func (m *mockImpersonationIdentity) String() string { return m.RoleArn }

// mockFailingIdentity always fails to unmarshal.
type mockFailingIdentity struct{}

func (*mockFailingIdentity) String() string             { return "" }
func (*mockFailingIdentity) UnmarshalJSON([]byte) error { return errors.New("mock unmarshal error") }

// mockBasicProvider implements only auth.Provider, not ProviderWithOIDCImpersonation
// or ProviderWithImpersonation.
type mockBasicProvider struct {
	returnName            string
	returnControllerToken auth.Token
}

func (m *mockBasicProvider) GetName() string { return m.returnName }
func (m *mockBasicProvider) NewControllerToken(_ context.Context, _ ...auth.Option) (auth.Token, error) {
	return m.returnControllerToken, nil
}

type mockProvider struct {
	t *testing.T

	returnName              string
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
	paramAudiences          []string
	paramServiceAccount     corev1.ServiceAccount
	paramOIDCTokenClient    *http.Client
	paramArtifactRepository string
	paramCluster            string
	paramClusterAddress     string
	paramAccessToken        auth.Token
	paramAccessTokens       []auth.Token
	paramAllowShellOut      bool

	// For multi-token flow (RESTConfig)
	paramFirstScopes   []string
	paramSecondScopes  []string
	expectFirstScopes  bool
	expectSecondScopes bool

	// Impersonation fields (used when wrapped with mockProviderWithImpersonation).
	returnImpersonationAnnotationKey  string
	returnIdentityForImpersonation    auth.Identity
	returnIdentityForImpersonationErr string
	returnImpersonatedToken           auth.Token
	returnImpersonateErr              error
	gotIdentity                       auth.Identity
}

// mockProviderWithImpersonation wraps mockProvider to also implement
// auth.ProviderWithImpersonation.
type mockProviderWithImpersonation struct {
	*mockProvider
}

func (m *mockProviderWithImpersonation) GetImpersonationAnnotationKey() string {
	if m.returnImpersonationAnnotationKey != "" {
		return m.returnImpersonationAnnotationKey
	}
	return "impersonation"
}

func (m *mockProviderWithImpersonation) NewIdentity() auth.Identity {
	if m.returnIdentityForImpersonationErr != "" {
		return &mockFailingIdentity{}
	}
	return &mockImpersonationIdentity{}
}

func (m *mockProviderWithImpersonation) NewTokenForNativeToken(ctx context.Context, token auth.Token,
	identity auth.Identity, opts ...auth.Option) (auth.Token, error) {
	m.gotIdentity = identity
	if m.returnImpersonateErr != nil {
		return nil, m.returnImpersonateErr
	}
	return m.returnImpersonatedToken, nil
}

func (m *mockProvider) GetName() string {
	return m.returnName
}

func (m *mockProvider) NewControllerToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {
	m.checkOptions(opts...)
	return m.returnControllerToken, nil
}

func (m *mockProvider) GetAudiences(_ context.Context, opts ...auth.Option) (string, string, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	var o auth.Options
	o.Apply(opts...)
	if o.ServiceAccount != nil {
		g.Expect(*o.ServiceAccount).To(Equal(m.paramServiceAccount))
	}
	return "mock-audience", "", nil
}

func (m *mockProvider) GetIdentity(opts ...auth.Option) (auth.Identity, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	var o auth.Options
	o.Apply(opts...)
	if o.ServiceAccount != nil {
		g.Expect(*o.ServiceAccount).To(Equal(m.paramServiceAccount))
	}
	if m.returnIdentityErr != "" {
		return nil, errors.New(m.returnIdentityErr)
	}
	return mockIdentity(m.returnIdentity), nil
}

func (m *mockProvider) NewTokenForOIDCToken(ctx context.Context, oidcToken, exchangeAudience string,
	targetIdentity auth.Identity, opts ...auth.Option) (auth.Token, error) {

	m.t.Helper()
	g := NewWithT(m.t)

	// Verify the OIDC token.
	token, _, err := jwt.NewParser().ParseUnverified(oidcToken, jwt.MapClaims{})
	g.Expect(err).NotTo(HaveOccurred())
	iss, err := token.Claims.GetIssuer()
	g.Expect(err).NotTo(HaveOccurred())
	ctx = oidc.ClientContext(ctx, m.paramOIDCTokenClient)
	jwks := oidc.NewRemoteKeySet(ctx, iss+"openid/v1/jwks")
	clientIDs := m.paramAudiences
	if len(clientIDs) == 0 {
		clientIDs = []string{"mock-audience"}
	}
	for _, aud := range clientIDs {
		_, err = oidc.NewVerifier(iss, jwks, &oidc.Config{
			ClientID:             aud,
			SupportedSigningAlgs: []string{token.Method.Alg()},
		}).Verify(ctx, oidcToken)
		g.Expect(err).NotTo(HaveOccurred())
	}

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

func (m *mockProvider) GetAccessTokenOptionsForCluster(opts ...auth.Option) ([][]auth.Option, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	var o auth.Options
	o.Apply(opts...)
	g.Expect(o.ClusterResource).To(Equal(m.paramCluster))
	if m.returnRESTConfigOptsErr != "" {
		return nil, errors.New(m.returnRESTConfigOptsErr)
	}
	return [][]auth.Option{{auth.WithScopes("first-token")}, {auth.WithScopes("second-token")}}, nil
}

func (m *mockProvider) NewRESTConfig(ctx context.Context, accessTokens []auth.Token,
	opts ...auth.Option) (*auth.RESTConfig, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	var o auth.Options
	o.Apply(opts...)
	g.Expect(o.ClusterResource).To(Equal(m.paramCluster))
	g.Expect(o.ClusterAddress).To(Equal(m.paramClusterAddress))
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
		switch {
		case m.expectFirstScopes:
			expectedScopes = m.paramFirstScopes
			m.expectFirstScopes = false
		case m.expectSecondScopes:
			expectedScopes = m.paramSecondScopes
			m.expectSecondScopes = false
		default:
			expectedScopes = []string{"scope1", "scope2"}
		}
	}

	expectedAudiences := []string{"audience1", "audience2"}
	if m.paramAudiences != nil {
		expectedAudiences = m.paramAudiences
	}
	g.Expect(o.Audiences).To(ConsistOf(expectedAudiences))
	g.Expect(o.Scopes).To(Equal(expectedScopes))
	g.Expect(o.STSRegion).To(Equal("us-east-1"))
	g.Expect(o.STSEndpoint).To(Equal("https://sts.some-cloud.io"))
	g.Expect(o.ProxyURL).To(Equal(&url.URL{Scheme: "http", Host: "proxy.io:8080"}))
	g.Expect(o.CAData).To(Equal("ca-data"))
	g.Expect(o.AllowShellOut).To(Equal(m.paramAllowShellOut))
}
