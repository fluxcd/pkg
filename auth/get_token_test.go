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
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
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
	*testing.T

	returnName              string
	returnAudience          string
	returnIdentity          string
	returnArtifactCacheKey  string
	returnDefaultToken      auth.Token
	returnAccessToken       auth.Token
	returnRegistryToken     auth.Token
	paramServiceAccount     corev1.ServiceAccount
	paramOIDCTokenClient    *http.Client
	paramArtifactRepository string
	paramAccessToken        auth.Token
}

func (m *mockProvider) GetName() string {
	return m.returnName
}

func (m *mockProvider) NewDefaultToken(ctx context.Context, opts ...auth.Option) (auth.Token, error) {
	return m.returnDefaultToken, nil
}

func (m *mockProvider) GetAudience(ctx context.Context) (string, error) {
	return m.returnAudience, nil
}

func (m *mockProvider) GetIdentity(serviceAccount corev1.ServiceAccount) (string, error) {
	m.Helper()
	g := NewWithT(m)
	g.Expect(serviceAccount).To(Equal(m.paramServiceAccount))
	return m.returnIdentity, nil
}

func (m *mockProvider) NewTokenForServiceAccount(ctx context.Context, oidcToken string,
	serviceAccount corev1.ServiceAccount, opts ...auth.Option) (auth.Token, error) {

	m.Helper()
	g := NewWithT(m)

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

	return m.returnAccessToken, nil
}

func (m *mockProvider) GetArtifactCacheKey(artifactRepository string) string {
	m.Helper()
	g := NewWithT(m)
	g.Expect(artifactRepository).To(Equal(m.paramArtifactRepository))
	return m.returnArtifactCacheKey
}

func (m *mockProvider) NewArtifactRegistryToken(ctx context.Context, artifactRepository string,
	accessToken auth.Token, opts ...auth.Option) (auth.Token, error) {
	m.Helper()
	g := NewWithT(m)
	g.Expect(artifactRepository).To(Equal(m.paramArtifactRepository))
	g.Expect(accessToken).To(Equal(m.paramAccessToken))
	return m.returnRegistryToken, nil
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
		name          string
		provider      *mockProvider
		opts          []auth.Option
		expectedToken auth.Token
	}{
		{
			name: "default access token (from controller)",
			provider: &mockProvider{
				returnDefaultToken: &mockToken{token: "mock-default-token"},
			},
			expectedToken: &mockToken{token: "mock-default-token"},
		},
		{
			name: "registry token from default access token (from controller)",
			provider: &mockProvider{
				returnDefaultToken:      &mockToken{token: "mock-default-token"},
				returnRegistryToken:     &mockToken{token: "mock-registry-token"},
				paramAccessToken:        &mockToken{token: "mock-default-token"},
				paramArtifactRepository: "some-registry.io/some/artifact",
			},
			opts: []auth.Option{
				auth.WithArtifactRepository("some-registry.io/some/artifact"),
			},
			expectedToken: &mockToken{token: "mock-registry-token"},
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
				returnName:              "mock-provider",
				returnAudience:          "mock-audience",
				returnAccessToken:       &mockToken{token: "mock-access-token"},
				returnRegistryToken:     &mockToken{token: "mock-registry-token"},
				paramServiceAccount:     *defaultServiceAccount,
				paramOIDCTokenClient:    oidcClient,
				paramArtifactRepository: "some-registry.io/some/artifact",
				paramAccessToken:        &mockToken{token: "mock-access-token"},
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithArtifactRepository("some-registry.io/some/artifact"),
			},
			expectedToken: &mockToken{token: "mock-registry-token"},
		},
		{
			name: "all the options are taken into account in the cache key",
			provider: &mockProvider{
				returnName:              "mock-provider",
				returnAudience:          "mock-audience",
				returnIdentity:          "mock-identity",
				returnArtifactCacheKey:  "artifact-cache-key",
				paramServiceAccount:     *defaultServiceAccount,
				paramArtifactRepository: "some-registry.io/some/artifact",
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithScopes("scope1", "scope2"),
				auth.WithArtifactRepository("some-registry.io/some/artifact"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())

					const key = "684363583b2fd97873df7e9089a6a47b9afea3007261d739b69c74e4364f1b02"
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
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tt.provider.T = t

			token, err := auth.GetToken(ctx, tt.provider, tt.opts...)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(token).To(Equal(tt.expectedToken))
		})
	}
}
