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
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
	. "github.com/onsi/gomega"
	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/actionsoidc"
	"github.com/fluxcd/pkg/auth/kubernetes"
	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/spiffe/brokerapi"
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

type mockBrokerAPIClient struct {
	t        *testing.T
	token    string
	calls    int
	refs     []brokerapi.KubernetesObjectReference
	requests [][]string
}

func (m *mockBrokerAPIClient) Close() error {
	return nil
}

func (m *mockBrokerAPIClient) FetchJWTSVID(_ context.Context, ref brokerapi.KubernetesObjectReference,
	audiences []string) (*jwtsvid.SVID, error) {

	m.t.Helper()
	m.calls++
	m.refs = append(m.refs, ref)
	m.requests = append(m.requests, audiences)
	return jwtsvid.ParseInsecure(m.token, audiences)
}

type fakeWorkloadAPI struct {
	workload.UnimplementedSpiffeWorkloadAPIServer

	mu       sync.Mutex
	token    string
	requests [][]string
}

func (f *fakeWorkloadAPI) FetchJWTSVID(_ context.Context,
	req *workload.JWTSVIDRequest) (*workload.JWTSVIDResponse, error) {

	f.mu.Lock()
	f.requests = append(f.requests, req.Audience)
	f.mu.Unlock()
	return &workload.JWTSVIDResponse{
		Svids: []*workload.JWTSVID{{
			SpiffeId: "spiffe://example.org/workload",
			Svid:     f.token,
		}},
	}, nil
}

func (f *fakeWorkloadAPI) Requests() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][]string(nil), f.requests...)
}

func startFakeWorkloadAPI(t *testing.T, fake *fakeWorkloadAPI) {
	t.Helper()

	dir, err := os.MkdirTemp("", "flux-spiffe")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	socketPath := filepath.Join(dir, "api.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen on socket: %v", err)
	}
	server := grpc.NewServer()
	workload.RegisterSpiffeWorkloadAPIServer(server, fake)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	t.Setenv(workloadapi.SocketEnv, "unix://"+socketPath)
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
	brokerAPIClient := &mockBrokerAPIClient{
		t:     t,
		token: newSPIFFEJWTSVID(t, []string{"audience1", "audience2"}, time.Hour),
	}
	provider := spiffeMockProvider(t)
	provider.paramOIDCToken = brokerAPIClient.token

	opts := spiffeOptions(object, brokerAPIClient)
	credential, err := auth.NewAccessCredential(ctx, provider, opts...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(credential).To(Equal(&mockCredential{token: "mock-access-token"}))
	g.Expect(brokerAPIClient.calls).To(Equal(1))
	g.Expect(brokerAPIClient.refs).To(Equal([]brokerapi.KubernetesObjectReference{{
		Group:     "source.toolkit.fluxcd.io",
		Kind:      "OCIRepository",
		Namespace: "flux-system",
		Name:      "object",
		UID:       "object-uid",
	}}))
	g.Expect(brokerAPIClient.requests).To(Equal([][]string{{"audience1", "audience2"}}))
}

func TestNewAccessCredentialSPIFFECacheKey(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	tokenCache, err := cache.NewTokenCache(3)
	g.Expect(err).NotTo(HaveOccurred())

	brokerAPIClient := &mockBrokerAPIClient{
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
			spiffeOptionsWithCache(objectA, brokerAPIClient, tokenCache)...)
		g.Expect(err).NotTo(HaveOccurred())
	}
	g.Expect(brokerAPIClient.calls).To(Equal(1))

	// Same ServiceAccount, different object: a new SPIFFE JWT-SVID is fetched.
	_, err = auth.NewAccessCredential(ctx, provider,
		spiffeOptionsWithCache(objectB, brokerAPIClient, tokenCache)...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(brokerAPIClient.calls).To(Equal(2))
	g.Expect(brokerAPIClient.refs[1].UID).To(Equal("object-b-uid"))
}

func TestNewAccessCredentialSPIFFEWorkloadAPI(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	token := newSPIFFEJWTSVID(t, []string{"audience1", "audience2"}, time.Hour)
	fake := &fakeWorkloadAPI{token: token}
	startFakeWorkloadAPI(t, fake)

	provider := spiffeWorkloadAPIMockProvider(t)
	provider.paramOIDCToken = token

	credential, err := auth.NewAccessCredential(ctx, provider,
		auth.WithIdentity("spiffe-workload"),
		auth.WithCredentialProvider(cryptov1.CredentialProviderSPIFFE),
		auth.WithCredentialType(cryptov1.CredentialTypeJWT),
		auth.WithCredentialAudiences("audience1", "audience2"),
		auth.WithScopes("scope1", "scope2"),
		auth.WithAWSRegion("us-east-1"),
		auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
		auth.WithCAData("ca-data"),
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(credential).To(Equal(&mockCredential{token: "mock-access-token"}))
	g.Expect(fake.Requests()).To(Equal([][]string{{"audience1", "audience2"}}))
}

func TestNewAccessCredentialSPIFFEWorkloadAPICache(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	token := newSPIFFEJWTSVID(t, []string{"audience1", "audience2"}, time.Hour)
	fake := &fakeWorkloadAPI{token: token}
	startFakeWorkloadAPI(t, fake)

	tokenCache, err := cache.NewTokenCache(3)
	g.Expect(err).NotTo(HaveOccurred())

	provider := spiffeWorkloadAPIMockProvider(t)
	provider.paramOIDCToken = token

	opts := []auth.Option{
		auth.WithIdentity("spiffe-workload"),
		auth.WithCredentialProvider(cryptov1.CredentialProviderSPIFFE),
		auth.WithCredentialType(cryptov1.CredentialTypeJWT),
		auth.WithCredentialAudiences("audience1", "audience2"),
		auth.WithScopes("scope1", "scope2"),
		auth.WithAWSRegion("us-east-1"),
		auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
		auth.WithCAData("ca-data"),
		auth.WithCache(*tokenCache),
	}

	// The first call fetches a JWT-SVID, the second is served from the cache.
	for range 2 {
		credential, err := auth.NewAccessCredential(ctx, provider, opts...)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(credential).To(Equal(&mockCredential{token: "mock-access-token"}))
	}
	g.Expect(fake.Requests()).To(HaveLen(1))
}

func TestNewAccessCredentialActionsOIDC(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	idToken := newTestJWT(t, time.Now().Add(time.Hour))

	var gotAudiences []string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAudiences = r.URL.Query()["audience"]
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":"` + idToken + `"}`))
	}))
	defer srv.Close()

	t.Setenv(actionsoidc.EnvRequestURL, srv.URL+"/token?api-version=2.0")
	t.Setenv(actionsoidc.EnvRequestToken, "request-token")

	object := actionsOIDCObject("object")
	credential, err := auth.NewAccessCredential(ctx, actionsOIDCMockProvider(t, idToken),
		actionsOIDCOptions(object)...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(credential).To(Equal(&mockCredential{token: "mock-access-token"}))
	g.Expect(gotAudiences).To(Equal([]string{"audience1", "audience2"}))
	g.Expect(gotAuth).To(Equal("bearer request-token"))
}

func TestNewAccessCredentialActionsOIDCErrors(t *testing.T) {
	ctx := context.Background()
	object := actionsOIDCObject("object")

	t.Run("missing environment variables", func(t *testing.T) {
		g := NewWithT(t)
		t.Setenv(actionsoidc.EnvRequestURL, "")
		t.Setenv(actionsoidc.EnvRequestToken, "")

		_, err := auth.NewAccessCredential(ctx, actionsOIDCMockProvider(t, ""),
			actionsOIDCOptions(object)...)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch Actions OIDC ID token")))
	})

	t.Run("token endpoint error", func(t *testing.T) {
		g := NewWithT(t)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte("denied"))
		}))
		defer srv.Close()

		t.Setenv(actionsoidc.EnvRequestURL, srv.URL)
		t.Setenv(actionsoidc.EnvRequestToken, "request-token")

		_, err := auth.NewAccessCredential(ctx, actionsOIDCMockProvider(t, ""),
			actionsOIDCOptions(object)...)
		g.Expect(err).To(MatchError(ContainSubstring("failed to fetch Actions OIDC ID token: " +
			"OIDC token request failed with status 403")))
	})
}

func TestNewAccessCredentialActionsOIDCCache(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	idToken := newTestJWT(t, time.Now().Add(time.Hour))

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"value":"` + idToken + `"}`))
	}))
	defer srv.Close()

	t.Setenv(actionsoidc.EnvRequestURL, srv.URL)
	t.Setenv(actionsoidc.EnvRequestToken, "request-token")

	tokenCache, err := cache.NewTokenCache(3)
	g.Expect(err).NotTo(HaveOccurred())

	provider := actionsOIDCMockProvider(t, idToken)
	opts := append(actionsOIDCOptions(actionsOIDCObject("object")), auth.WithCache(*tokenCache))

	// The first call fetches a token, the second is served from the cache.
	for range 2 {
		credential, err := auth.NewAccessCredential(ctx, provider, opts...)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(credential).To(Equal(&mockCredential{token: "mock-access-token"}))
	}
	g.Expect(calls).To(Equal(1))
}

func actionsOIDCObject(name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "flux-system",
			UID:       "object-uid",
		},
	}
}

func actionsOIDCMockProvider(t *testing.T, idToken string) *mockProvider {
	return &mockProvider{
		t:                       t,
		returnName:              "mock-provider",
		returnIdentity:          "mock-identity",
		returnAccessCredential:  &mockCredential{token: "mock-access-token"},
		paramAudiences:          []string{"audience1", "audience2"},
		paramCredentialProvider: cryptov1.CredentialProviderActionsOIDC,
		paramOIDCToken:          idToken,
		skipOIDCValidation:      true,
	}
}

func actionsOIDCOptions(object *corev1.ServiceAccount) []auth.Option {
	return []auth.Option{
		auth.WithObject("source.toolkit.fluxcd.io", "OCIRepository", object),
		auth.WithIdentity("flux-mirror"),
		auth.WithCredentialProvider(cryptov1.CredentialProviderActionsOIDC),
		auth.WithCredentialType(cryptov1.CredentialTypeJWT),
		auth.WithCredentialAudiences("audience1", "audience2"),
		auth.WithScopes("scope1", "scope2"),
		auth.WithAWSRegion("us-east-1"),
		auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
		auth.WithCAData("ca-data"),
	}
}

// newTestJWT mints an unsigned-but-parseable JWT carrying the given exp claim.
func newTestJWT(t *testing.T, exp time.Time) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"exp": exp.Unix()})
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("failed to mint test JWT: %v", err)
	}
	return signed
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

func spiffeWorkloadAPIMockProvider(t *testing.T) *mockProvider {
	return &mockProvider{
		t:                      t,
		returnName:             "mock-provider",
		returnIdentity:         "mock-identity",
		returnAccessCredential: &mockCredential{token: "mock-access-token"},
		paramAudiences:         []string{"audience1", "audience2"},
		skipOIDCValidation:     true,
	}
}

func spiffeOptions(object *corev1.ServiceAccount, brokerAPIClient brokerapi.Interface) []auth.Option {
	return []auth.Option{
		auth.WithServiceAccountName("spiffe-sa"),
		auth.WithObject("source.toolkit.fluxcd.io", "OCIRepository", object),
		auth.WithCredentialProvider(cryptov1.CredentialProviderSPIFFE),
		auth.WithCredentialType(cryptov1.CredentialTypeJWT),
		auth.WithCredentialAudiences("audience1", "audience2"),
		auth.WithSPIFFEBrokerAPI(brokerAPIClient),
		auth.WithScopes("scope1", "scope2"),
		auth.WithAWSRegion("us-east-1"),
		auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
		auth.WithCAData("ca-data"),
	}
}

func spiffeOptionsWithCache(object *corev1.ServiceAccount, brokerAPIClient brokerapi.Interface,
	tokenCache *cache.TokenCache) []auth.Option {
	return append(spiffeOptions(object, brokerAPIClient), auth.WithCache(*tokenCache))
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
