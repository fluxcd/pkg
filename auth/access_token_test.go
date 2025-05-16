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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
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
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
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
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				auth.WithAllowShellOut(),
			},
			expectedToken: &mockToken{token: "mock-default-token"},
		},
		{
			name: "access token from service account",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  *defaultServiceAccount,
				paramOIDCTokenClient: oidcClient,
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
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
			name: "access token from service account - default audience",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{},
				paramServiceAccount:  *defaultServiceAccount,
				paramOIDCTokenClient: oidcClient,
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
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
				paramServiceAccount: *defaultServiceAccount,
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())

					const key = "db625bd5a96dc48fcc100659c6db98857d1e0ceec930bbded0fdece14af4307c"
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
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
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
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())
					o.Cache = tokenCache
				},
			},
			expectedErr: "failed to get provider identity from service account 'default/default' annotations: mock error",
		},
		{
			name: "disable object level workload identity",
			provider: &mockProvider{
				paramServiceAccount: *defaultServiceAccount,
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
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

			token, err := auth.GetAccessToken(ctx, tt.provider, tt.opts...)

			if tt.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.expectedErr))
				g.Expect(token).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(tt.expectedToken))
			}
		})
	}
}
