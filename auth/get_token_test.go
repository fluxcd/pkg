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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/cache"
)

type mockToken struct {
	token string
}

func (m *mockToken) GetDuration() time.Duration {
	return time.Hour
}

type mockProvider struct {
	t *testing.T

	returnName              string
	returnAudience          string
	returnIdentity          string
	returnIdentityErr       string
	returnRegistryErr       string
	returnRegistryInput     string
	returnControllerToken   auth.Token
	returnAccessToken       auth.Token
	returnRegistryToken     *auth.ArtifactRegistryCredentials
	paramServiceAccount     corev1.ServiceAccount
	paramOIDCTokenClient    *http.Client
	paramArtifactRepository string
	paramAccessToken        auth.Token
}

func (m *mockProvider) GetName() string {
	return m.returnName
}

func (m *mockProvider) NewControllerToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {
	checkOptions(m.t, opts...)
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

	checkOptions(m.t, opts...)

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

func (m *mockProvider) NewArtifactRegistryCredentials(ctx context.Context, registryInput string,
	accessToken auth.Token, opts ...auth.Option) (*auth.ArtifactRegistryCredentials, error) {
	m.t.Helper()
	g := NewWithT(m.t)
	g.Expect(registryInput).To(Equal(m.paramArtifactRepository))
	g.Expect(accessToken).To(Equal(m.paramAccessToken))
	checkOptions(m.t, opts...)
	return m.returnRegistryToken, nil
}

func checkOptions(t *testing.T, opts ...auth.Option) {
	t.Helper()
	g := NewWithT(t)

	var o auth.Options
	o.Apply(opts...)

	g.Expect(o.Scopes).To(Equal([]string{"scope1", "scope2"}))
	g.Expect(o.STSRegion).To(Equal("us-east-1"))
	g.Expect(o.STSEndpoint).To(Equal("https://sts.some-cloud.io"))
	g.Expect(o.ProxyURL).To(Equal(&url.URL{Scheme: "http", Host: "proxy.io:8080"}))
}

func TestGetToken(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	// Create test env.
	testEnv := &envtest.Environment{}
	conf, err := testEnv.Start()
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { testEnv.Stop() })
	kubeClient, err := client.New(conf, client.Options{})
	g.Expect(err).NotTo(HaveOccurred())

	// Create HTTP client for OIDC verification.
	clusterCAPool := x509.NewCertPool()
	ok := clusterCAPool.AppendCertsFromPEM(conf.TLSClientConfig.CAData)
	g.Expect(ok).To(BeTrue())
	oidcClient := &http.Client{}
	oidcClient.Transport = http.DefaultTransport.(*http.Transport).Clone()
	oidcClient.Transport.(*http.Transport).TLSClientConfig = &tls.Config{
		RootCAs: clusterCAPool,
	}

	// Grant anonymous access to service account issuer discovery.
	err = kubeClient.Create(ctx, &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "anonymous-service-account-issuer-discovery",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "User",
				Name:     "system:anonymous",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "system:service-account-issuer-discovery",
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	// Create a default service account.
	defaultServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}
	err = kubeClient.Create(ctx, defaultServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())
	saRef := client.ObjectKey{
		Name:      defaultServiceAccount.Name,
		Namespace: defaultServiceAccount.Namespace,
	}

	for _, tt := range []struct {
		name               string
		provider           *mockProvider
		opts               []auth.Option
		disableObjectLevel bool
		expectedToken      auth.Token
		expectedErr        string
	}{
		{
			name: "controller access token",
			provider: &mockProvider{
				returnControllerToken: &mockToken{token: "mock-default-token"},
			},
			opts: []auth.Option{
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
			},
			expectedToken: &mockToken{token: "mock-default-token"},
		},
		{
			name: "registry token from controller access token",
			provider: &mockProvider{
				returnRegistryInput:   "some-registry.io/some/artifact",
				returnControllerToken: &mockToken{token: "mock-default-token"},
				returnRegistryToken: &auth.ArtifactRegistryCredentials{
					Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
				},
				paramAccessToken:        &mockToken{token: "mock-default-token"},
				paramArtifactRepository: "some-registry.io/some/artifact",
			},
			opts: []auth.Option{
				auth.WithArtifactRepository("some-registry.io/some/artifact"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
			},
			expectedToken: &auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
			},
		},
		{
			name: "access token from service account",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAudience:       "mock-audience",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramServiceAccount:  *defaultServiceAccount,
				paramOIDCTokenClient: oidcClient,
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				// Exercise the code path where a cache is set but no token is
				// available in the cache.
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())
					o.Cache = tokenCache
				},
			},
			expectedToken: &mockToken{token: "mock-access-token"},
		},
		{
			name: "registry token from access token from service account",
			provider: &mockProvider{
				returnName:          "mock-provider",
				returnAudience:      "mock-audience",
				returnRegistryInput: "some-registry.io/some/artifact",
				returnAccessToken:   &mockToken{token: "mock-access-token"},
				returnRegistryToken: &auth.ArtifactRegistryCredentials{
					Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
				},
				paramServiceAccount:     *defaultServiceAccount,
				paramOIDCTokenClient:    oidcClient,
				paramArtifactRepository: "some-registry.io/some/artifact",
				paramAccessToken:        &mockToken{token: "mock-access-token"},
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithArtifactRepository("some-registry.io/some/artifact"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
			},
			expectedToken: &auth.ArtifactRegistryCredentials{
				Authenticator: authn.FromConfig(authn.AuthConfig{Username: "mock-registry-token"}),
			},
		},
		{
			name: "all the options are taken into account in the cache key",
			provider: &mockProvider{
				returnName:              "mock-provider",
				returnAudience:          "mock-audience",
				returnIdentity:          "mock-identity",
				returnRegistryInput:     "artifact-cache-key",
				paramServiceAccount:     *defaultServiceAccount,
				paramArtifactRepository: "some-registry.io/some/artifact",
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithScopes("scope1", "scope2"),
				auth.WithArtifactRepository("some-registry.io/some/artifact"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())

					const key = "3e8e270134e99fda1a01d7dca77f29448eb4c7f6cc026137b85a1bcd96b276fa"
					token := &mockToken{token: "cached-token"}
					cachedToken, ok, err := tokenCache.GetOrSet(ctx, key, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedToken).To(Equal(token))

					o.Cache = tokenCache
				},
			},
			expectedToken: &mockToken{token: "cached-token"},
		},
		{
			name: "error getting identity",
			provider: &mockProvider{
				returnIdentityErr:   "mock error",
				paramServiceAccount: *defaultServiceAccount,
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
			},
			expectedErr: "failed to get provider identity from service account 'default/default' annotations: mock error",
		},
		{
			name: "error getting identity using cache",
			provider: &mockProvider{
				returnIdentityErr:   "mock error",
				paramServiceAccount: *defaultServiceAccount,
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())
					o.Cache = tokenCache
				},
			},
			expectedErr: "failed to get provider identity from service account 'default/default' annotations: mock error",
		},
		{
			name: "error parsing artifact repository",
			provider: &mockProvider{
				paramArtifactRepository: "some-registry.io/some/artifact",
				returnRegistryErr:       "mock error",
			},
			opts: []auth.Option{
				auth.WithArtifactRepository("some-registry.io/some/artifact"),
			},
			expectedErr: "failed to parse artifact repository 'some-registry.io/some/artifact': mock error",
		},
		{
			name:     "disable object level workload identity",
			provider: &mockProvider{},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
			},
			disableObjectLevel: true,
			expectedErr:        "ObjectLevelWorkloadIdentity feature gate is not enabled",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tt.provider.t = t

			if !tt.disableObjectLevel {
				t.Setenv(auth.EnvVarEnableObjectLevelWorkloadIdentity, "true")
			}

			token, err := auth.GetToken(ctx, tt.provider, tt.opts...)

			if tt.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(tt.expectedToken))
			}
		})
	}
}
