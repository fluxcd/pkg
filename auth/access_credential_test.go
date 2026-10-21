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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	. "github.com/onsi/gomega"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/kubernetes"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/spiffe/broker"
)

func TestNewAccessCredential(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	kubeClient, oidcClient := newTestEnv(t, ctx)

	// Create a default service account.
	defaultServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "default",
		},
	}
	err := kubeClient.Create(ctx, defaultServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a lockdown service account for testing lockdown functionality.
	lockdownServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lockdown-sa",
			Namespace: "default",
		},
	}
	err = kubeClient.Create(ctx, lockdownServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	for _, tt := range []struct {
		name               string
		provider           *mockProvider
		opts               []auth.Option
		expectedCredential auth.Credential
		expectedErr        string
	}{
		{
			name: "controller access credential",
			provider: &mockProvider{
				returnControllerCredential: &mockCredential{token: "mock-default-token"},
			},
			opts: []auth.Option{
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCredential: &mockCredential{token: "mock-default-token"},
		},
		{
			name: "controller access credential allowing shell out",
			provider: &mockProvider{
				returnControllerCredential: &mockCredential{token: "mock-default-token"},
				paramAllowShellOut:         true,
			},
			opts: []auth.Option{
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				auth.WithAllowShellOut(),
			},
			expectedCredential: &mockCredential{token: "mock-default-token"},
		},
		{
			name: "access credential from explicit service account",
			provider: &mockProvider{
				returnName:             "mock-provider",
				returnAccessCredential: &mockCredential{token: "mock-access-token"},
				paramAudiences:         []string{"audience1", "audience2"},
				paramServiceAccount:    &kubernetes.ServiceAccountReference{Name: lockdownServiceAccount.Name, Namespace: lockdownServiceAccount.Namespace},
				paramOIDCTokenClient:   oidcClient,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(lockdownServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", lockdownServiceAccount),
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCredential: &mockCredential{token: "mock-access-token"},
		},
		{
			name: "access credential from service account",
			provider: &mockProvider{
				returnName:             "mock-provider",
				returnAccessCredential: &mockCredential{token: "mock-access-token"},
				paramAudiences:         []string{"audience1", "audience2"},
				paramServiceAccount:    &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient:   oidcClient,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(defaultServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", defaultServiceAccount),
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				// Exercise the code path where a cache is set but no token is
				// available in the cache.
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())
					o.Cache = tokenCache
				},
			},
			expectedCredential: &mockCredential{token: "mock-access-token"},
		},
		{
			name: "access credential from service account with credential expiration",
			provider: &mockProvider{
				returnName:             "mock-provider",
				returnAccessCredential: &mockCredential{token: "mock-access-token"},
				paramAudiences:         []string{"audience1"},
				paramServiceAccount:    &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient:   oidcClient,
				paramOIDCTokenExpiry:   15 * time.Minute,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(defaultServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", defaultServiceAccount),
				auth.WithCredentialAudiences("audience1"),
				auth.WithCredentialExpiration(15 * time.Minute),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCredential: &mockCredential{token: "mock-access-token"},
		},
		{
			name: "access credential from service account with explicit name ignoring default",
			provider: &mockProvider{
				returnName:             "mock-provider",
				returnAccessCredential: &mockCredential{token: "mock-access-token"},
				paramAudiences:         []string{"audience1", "audience2"},
				paramServiceAccount:    &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient:   oidcClient,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(defaultServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", defaultServiceAccount),
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCredential: &mockCredential{token: "mock-access-token"},
		},
		{
			name: "access credential from service account - default audience",
			provider: &mockProvider{
				returnName:             "mock-provider",
				returnAccessCredential: &mockCredential{token: "mock-access-token"},
				paramAudiences:         []string{},
				paramServiceAccount:    &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient:   oidcClient,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(defaultServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", defaultServiceAccount),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCredential: &mockCredential{token: "mock-access-token"},
		},
		{
			name: "all the options are taken into account in the cache key",
			provider: &mockProvider{
				returnName:          "mock-provider",
				returnIdentity:      "mock-identity",
				paramAudiences:      []string{"audience1", "audience2"},
				paramServiceAccount: &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(defaultServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", defaultServiceAccount),
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())

					const key = "6c016e84cc74ad7bf6bd8770cdb6a2efca3e185fdf90badd985c77984784f5c3"
					token := &mockCredential{token: "cached-token"}
					cachedCredential, ok, err := tokenCache.GetOrSet(ctx, key, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedCredential).To(Equal(token))

					o.Cache = tokenCache
				},
			},
			expectedCredential: &mockCredential{token: "cached-token"},
		},
		{
			name: "error getting identity",
			provider: &mockProvider{
				returnIdentityErr:   "mock error",
				paramServiceAccount: &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(defaultServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", defaultServiceAccount),
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedErr: "failed to get provider identity from service account 'default/default': mock error",
		},
		{
			name: "error getting identity using cache",
			provider: &mockProvider{
				returnIdentityErr:   "mock error",
				paramServiceAccount: &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(defaultServiceAccount.Name),
				auth.WithObject("", "ServiceAccount", defaultServiceAccount),
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())
					o.Cache = tokenCache
				},
			},
			expectedErr: "failed to get provider identity from service account 'default/default': mock error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tt.provider.t = t

			credential, err := auth.NewAccessCredential(ctx, tt.provider, tt.opts...)

			if tt.expectedErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.expectedErr)))
				g.Expect(credential).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(credential).To(Equal(tt.expectedCredential))
			}
		})
	}
}

// TestNewAccessCredential_ControllerLevelAudienceCacheKey is a regression test for a
// credential cache collision: controller-level (no service account) access credentials are
// also audience-scoped (e.g. SOPS Vault/OpenBao decryption and kubeconfig
// ServiceAccount credentials), so requesting a credential for one audience must never
// return a cached credential that was minted for a different audience.
func TestNewAccessCredential_ControllerLevelAudienceCacheKey(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	tokenCache, err := cache.NewTokenCache(3)
	g.Expect(err).NotTo(HaveOccurred())

	provider := &mockProvider{t: t, returnName: "mock-provider"}

	// Controller-level options (no service account) for a given audience. The
	// non-audience options exist only to satisfy the mock provider's checks.
	opts := func(audience string) []auth.Option {
		provider.paramAudiences = []string{audience}
		return []auth.Option{
			auth.WithCredentialAudiences(audience),
			auth.WithScopes("scope1", "scope2"),
			auth.WithAWSRegion("us-east-1"),
			auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
			auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
			auth.WithCAData("ca-data"),
			auth.WithObject("", "test", &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
			}),
			auth.WithCache(*tokenCache),
		}
	}

	// Mint and cache a controller credential for the first audience.
	provider.returnControllerCredential = &mockCredential{token: "token-audience-a"}
	got, err := auth.NewAccessCredential(ctx, provider, opts("audience-a")...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(&mockCredential{token: "token-audience-a"}))

	// A different audience must NOT reuse the first audience's cached credential.
	provider.returnControllerCredential = &mockCredential{token: "token-audience-b"}
	got, err = auth.NewAccessCredential(ctx, provider, opts("audience-b")...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(&mockCredential{token: "token-audience-b"}))

	// The first audience still returns its own cached credential (per-audience cache);
	// the refreshed controller credential must not be minted.
	provider.returnControllerCredential = &mockCredential{token: "token-audience-a-fresh"}
	got, err = auth.NewAccessCredential(ctx, provider, opts("audience-a")...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(&mockCredential{token: "token-audience-a"}))
}

type mockBrokerClient struct {
	t        *testing.T
	token    string
	calls    int
	refs     []broker.KubernetesObjectReference
	requests [][]string
}

func (m *mockBrokerClient) Close() error {
	return nil
}

func (m *mockBrokerClient) FetchJWTSVID(_ context.Context, ref broker.KubernetesObjectReference,
	audiences []string) (*jwtsvid.SVID, error) {

	m.t.Helper()
	m.calls++
	m.refs = append(m.refs, ref)
	m.requests = append(m.requests, audiences)
	return jwtsvid.ParseInsecure(m.token, audiences)
}

func TestNewAccessCredentialSPIFFE(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	object := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "object",
			Namespace: "flux-system",
			UID:       "object-uid",
		},
	}
	brokerClient := &mockBrokerClient{
		t:     t,
		token: newSPIFFEJWTSVID(t, []string{"audience1", "audience2"}, time.Hour),
	}
	provider := spiffeMockProvider(t)
	provider.paramOIDCToken = brokerClient.token

	opts := spiffeOptions(object, brokerClient)
	credential, err := auth.NewAccessCredential(ctx, provider, opts...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(credential).To(Equal(&mockCredential{token: "mock-access-token"}))
	g.Expect(brokerClient.calls).To(Equal(1))
	g.Expect(brokerClient.refs).To(Equal([]broker.KubernetesObjectReference{{
		Group:     "source.toolkit.fluxcd.io",
		Kind:      "OCIRepository",
		Namespace: "flux-system",
		Name:      "object",
		UID:       "object-uid",
	}}))
	g.Expect(brokerClient.requests).To(Equal([][]string{{"audience1", "audience2"}}))
}

func TestNewAccessCredentialSPIFFECacheKey(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	tokenCache, err := cache.NewTokenCache(3)
	g.Expect(err).NotTo(HaveOccurred())

	brokerClient := &mockBrokerClient{
		t:     t,
		token: newSPIFFEJWTSVID(t, []string{"audience1", "audience2"}, time.Hour),
	}
	provider := spiffeMockProvider(t)

	objectA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "object-a",
			Namespace: "flux-system",
			UID:       "object-a-uid",
		},
	}
	objectB := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "object-b",
			Namespace: "flux-system",
			UID:       "object-b-uid",
		},
	}

	// Same object: the second call is served from the cache.
	for range 2 {
		_, err := auth.NewAccessCredential(ctx, provider,
			spiffeOptionsWithCache(objectA, brokerClient, tokenCache)...)
		g.Expect(err).NotTo(HaveOccurred())
	}
	g.Expect(brokerClient.calls).To(Equal(1))

	// Same ServiceAccount, different object: a new SPIFFE JWT-SVID is fetched.
	_, err = auth.NewAccessCredential(ctx, provider,
		spiffeOptionsWithCache(objectB, brokerClient, tokenCache)...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(brokerClient.calls).To(Equal(2))
	g.Expect(brokerClient.refs[1].UID).To(Equal("object-b-uid"))
}

func TestNewAccessCredentialSPIFFERequiresBroker(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	object := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "object",
			Namespace: "flux-system",
			UID:       "object-uid",
		},
	}

	opts := spiffeOptions(object, nil)
	_, err := auth.NewAccessCredential(ctx, spiffeMockProvider(t), opts...)
	g.Expect(err).To(MatchError("SPIFFE Broker client is required for the SPIFFE credential provider"))
}

func spiffeMockProvider(t *testing.T) *mockProvider {
	return &mockProvider{
		t:                      t,
		returnName:             "mock-provider",
		returnIdentity:         "mock-identity",
		returnAccessCredential: &mockCredential{token: "mock-access-token"},
		paramAudiences:         []string{"audience1", "audience2"},
		paramServiceAccount: &kubernetes.ServiceAccountReference{
			Name:      "spiffe-sa",
			Namespace: "flux-system",
		},
		skipOIDCValidation: true,
	}
}

func spiffeOptions(object *corev1.ServiceAccount, brokerClient broker.Interface) []auth.Option {
	return []auth.Option{
		auth.WithServiceAccountName("spiffe-sa"),
		auth.WithObject("source.toolkit.fluxcd.io", "OCIRepository", object),
		auth.WithCredentialProvider(cryptov1.CredentialProviderSPIFFE),
		auth.WithCredentialType(cryptov1.CredentialTypeJWT),
		auth.WithCredentialAudiences("audience1", "audience2"),
		auth.WithSPIFFEBroker(brokerClient),
		auth.WithScopes("scope1", "scope2"),
		auth.WithAWSRegion("us-east-1"),
		auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
		auth.WithCAData("ca-data"),
	}
}

func spiffeOptionsWithCache(object *corev1.ServiceAccount, brokerClient broker.Interface,
	tokenCache *cache.TokenCache) []auth.Option {
	return append(spiffeOptions(object, brokerClient), auth.WithCache(*tokenCache))
}

func newSPIFFEJWTSVID(t *testing.T, audiences []string, ttl time.Duration) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate JWT key: %v", err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, nil)
	if err != nil {
		t.Fatalf("failed to create JWT signer: %v", err)
	}
	claims, err := json.Marshal(map[string]any{
		"sub": "spiffe://example.org/ns/flux-system/ocirepository/object",
		"aud": audiences,
		"exp": time.Now().Add(ttl).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("failed to marshal JWT claims: %v", err)
	}
	jws, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	token, err := jws.CompactSerialize()
	if err != nil {
		t.Fatalf("failed to serialize JWT: %v", err)
	}
	return token
}
