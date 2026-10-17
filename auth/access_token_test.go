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
	"net/url"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/kubernetes"
	"github.com/fluxcd/pkg/cache"
)

func TestGetAccessToken(t *testing.T) {
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
		name          string
		provider      *mockProvider
		opts          []auth.Option
		expectedToken auth.Token
		expectedErr   string
	}{
		{
			name: "controller access token",
			provider: &mockProvider{
				returnControllerToken: &mockToken{token: "mock-default-token"},
			},
			opts: []auth.Option{
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedToken: &mockToken{token: "mock-default-token"},
		},
		{
			name: "controller access token allowing shell out",
			provider: &mockProvider{
				returnControllerToken: &mockToken{token: "mock-default-token"},
				paramAllowShellOut:    true,
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
			expectedToken: &mockToken{token: "mock-default-token"},
		},
		{
			name: "access token from explicit service account",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  &kubernetes.ServiceAccountReference{Name: lockdownServiceAccount.Name, Namespace: lockdownServiceAccount.Namespace},
				paramOIDCTokenClient: oidcClient,
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
			expectedToken: &mockToken{token: "mock-access-token"},
		},
		{
			name: "access token from service account",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient: oidcClient,
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
			expectedToken: &mockToken{token: "mock-access-token"},
		},
		{
			name: "access token from service account with credential expiration",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{"audience1"},
				paramServiceAccount:  &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient: oidcClient,
				paramOIDCTokenExpiry: 15 * time.Minute,
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
			expectedToken: &mockToken{token: "mock-access-token"},
		},
		{
			name: "access token from service account with explicit name ignoring default",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient: oidcClient,
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
			expectedToken: &mockToken{token: "mock-access-token"},
		},
		{
			name: "access token from service account - default audience",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{},
				paramServiceAccount:  &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient: oidcClient,
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
			expectedToken: &mockToken{token: "mock-access-token"},
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

			token, err := auth.GetAccessToken(ctx, tt.provider, tt.opts...)

			if tt.expectedErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.expectedErr)))
				g.Expect(token).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(tt.expectedToken))
			}
		})
	}
}

// TestGetAccessToken_ControllerLevelAudienceCacheKey is a regression test for a
// token cache collision: controller-level (no service account) access tokens are
// also audience-scoped (e.g. SOPS Vault/OpenBao decryption and kubeconfig
// ServiceAccount tokens), so requesting a token for one audience must never
// return a cached token that was minted for a different audience.
func TestGetAccessToken_ControllerLevelAudienceCacheKey(t *testing.T) {
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

	// Mint and cache a controller token for the first audience.
	provider.returnControllerToken = &mockToken{token: "token-audience-a"}
	got, err := auth.GetAccessToken(ctx, provider, opts("audience-a")...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(&mockToken{token: "token-audience-a"}))

	// A different audience must NOT reuse the first audience's cached token.
	provider.returnControllerToken = &mockToken{token: "token-audience-b"}
	got, err = auth.GetAccessToken(ctx, provider, opts("audience-b")...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(&mockToken{token: "token-audience-b"}))

	// The first audience still returns its own cached token (per-audience cache);
	// the refreshed controller token must not be minted.
	provider.returnControllerToken = &mockToken{token: "token-audience-a-fresh"}
	got, err = auth.GetAccessToken(ctx, provider, opts("audience-a")...)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(got).To(Equal(&mockToken{token: "token-audience-a"}))
}
