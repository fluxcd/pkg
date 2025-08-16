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
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/cache"
)

func TestParseClusterAddress(t *testing.T) {
	tests := []struct {
		address  string
		expected string
		err      string
	}{
		{
			address:  "https://example.com:443",
			expected: "https://example.com:443",
		},
		{
			address:  "example.com",
			expected: "https://example.com:443",
		},
		{
			address:  "EXAMPLE.COM:8080",
			expected: "https://example.com:8080",
		},
		{
			address:  "34.44.60.80",
			expected: "https://34.44.60.80:443",
		},
		{
			address: "",
			err:     "empty address",
		},
		{
			address: "------------\t",
			err:     "failed to parse Kubernetes API server address 'https://------------	':",
		},
		{
			address: "http://example.com:443",
			err:     "Kubernetes API server address 'http://example.com:443' must use https scheme",
		},
	}

	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.address, "/", ""), func(t *testing.T) {
			g := NewWithT(t)

			address, err := auth.ParseClusterAddress(tt.address)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(address).To(BeEmpty())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(address).To(Equal(tt.expected))
			}
		})
	}
}

func TestGetRESTConfig(t *testing.T) {
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

	// Create a lockdown service account for testing lockdown functionality.
	lockdownServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lockdown-sa",
			Namespace: "default",
		},
	}
	err = kubeClient.Create(ctx, lockdownServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	now := time.Now()

	for _, tt := range []struct {
		name                string
		provider            *mockProvider
		cluster             string
		opts                []auth.Option
		disableObjectLevel  bool
		defaultKubeConfigSA string
		expectedCreds       *auth.RESTConfig
		expectedErr         string
	}{
		{
			name: "restconfig from controller access token",
			provider: &mockProvider{
				returnControllerToken: &mockToken{token: "mock-default-token"},
				returnRESTConfig: &auth.RESTConfig{
					Host:        "https://cluster/resource/name",
					BearerToken: "mock-bearer-token",
					CAData:      []byte("ca-data"),
				},
				paramCluster:       "cluster/resource/name",
				paramFirstScopes:   []string{"first-token"},
				paramSecondScopes:  []string{"second-token"},
				expectFirstScopes:  true,
				expectSecondScopes: true,
				paramAccessTokens: []auth.Token{
					&mockToken{token: "mock-default-token"},
					&mockToken{token: "mock-default-token"},
				},
			},
			cluster: "cluster/resource/name",
			opts: []auth.Option{
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCreds: &auth.RESTConfig{
				Host:        "https://cluster/resource/name",
				BearerToken: "mock-bearer-token",
				CAData:      []byte("ca-data"),
			},
		},
		{
			name: "restconfig from access token from service account",
			provider: &mockProvider{
				returnName:        "mock-provider",
				returnAccessToken: &mockToken{token: "mock-access-token"},
				returnRESTConfig: &auth.RESTConfig{
					Host:        "https://cluster/resource/name",
					BearerToken: "mock-bearer-token",
					CAData:      []byte("ca-data"),
				},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  *defaultServiceAccount,
				paramOIDCTokenClient: oidcClient,
				paramCluster:         "cluster/resource/name",
				paramFirstScopes:     []string{"first-token"},
				paramSecondScopes:    []string{"second-token"},
				expectFirstScopes:    true,
				expectSecondScopes:   true,
				paramAccessTokens: []auth.Token{
					&mockToken{token: "mock-access-token"},
					&mockToken{token: "mock-access-token"},
				},
			},
			cluster: "cluster/resource/name",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(saRef.Name),
				auth.WithServiceAccountNamespace(saRef.Namespace),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCreds: &auth.RESTConfig{
				Host:        "https://cluster/resource/name",
				BearerToken: "mock-bearer-token",
				CAData:      []byte("ca-data"),
			},
		},
		{
			name: "all the options are taken into account in the cache key",
			provider: &mockProvider{
				returnName:          "mock-provider",
				returnIdentity:      "mock-identity",
				paramAudiences:      []string{"audience1", "audience2"},
				paramServiceAccount: *defaultServiceAccount,
				paramCluster:        "cluster/resource/name",
				paramClusterAddress: "https://cluster/resource/name",
				paramFirstScopes:    []string{"first-token"},
				paramSecondScopes:   []string{"second-token"},
				expectFirstScopes:   true,
				expectSecondScopes:  true,
			},
			cluster: "cluster/resource/name",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(saRef.Name),
				auth.WithServiceAccountNamespace(saRef.Namespace),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				auth.WithClusterResource("cluster/resource/name"),
				auth.WithClusterAddress("https://cluster/resource/name"),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(3)
					g.Expect(err).NotTo(HaveOccurred())

					accessTokenKey := "500a3116f5d1c492d7a5ea97cdf9a7f869815346c79f01c7368703c241ebb5eb"
					var token auth.Token = &mockToken{token: "cached-token"}
					cachedToken, ok, err := tokenCache.GetOrSet(ctx, accessTokenKey, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedToken).To(Equal(token))

					accessTokenKey = "0b1167fc851943c6153d40e149cd2970aac121aaf03b1fcad158672974f58827"
					token = &mockToken{token: "cached-token"}
					cachedToken, ok, err = tokenCache.GetOrSet(ctx, accessTokenKey, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedToken).To(Equal(token))

					const restConfigKey = "a1937b7b1df13ac8ad784db686088c4cd5b4c4877318d07d3fa19ab8caf9d7c2"
					token = &auth.RESTConfig{
						Host:        "https://cluster/resource/name",
						BearerToken: "mock-bearer-token",
						CAData:      []byte("ca-data"),
						ExpiresAt:   now.Add(time.Hour),
					}
					cachedToken, ok, err = tokenCache.GetOrSet(ctx, restConfigKey, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedToken).To(Equal(token))

					o.Cache = tokenCache
				},
			},
			expectedCreds: &auth.RESTConfig{
				Host:        "https://cluster/resource/name",
				BearerToken: "mock-bearer-token",
				CAData:      []byte("ca-data"),
				ExpiresAt:   now.Add(time.Hour),
			},
		},
		{
			name: "error getting access token options for cluster",
			provider: &mockProvider{
				paramCluster:            "cluster/resource/name",
				returnRESTConfigOptsErr: "mock error",
			},
			cluster:     "cluster/resource/name",
			expectedErr: "mock error",
		},
		{
			name: "restconfig from default kubeconfig service account using lockdown support",
			provider: &mockProvider{
				returnName:        "mock-provider",
				returnAccessToken: &mockToken{token: "mock-access-token"},
				returnRESTConfig: &auth.RESTConfig{
					Host:        "https://cluster/resource/name",
					BearerToken: "mock-bearer-token",
					CAData:      []byte("ca-data"),
				},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  *lockdownServiceAccount,
				paramOIDCTokenClient: oidcClient,
				paramCluster:         "cluster/resource/name",
				paramFirstScopes:     []string{"first-token"},
				paramSecondScopes:    []string{"second-token"},
				expectFirstScopes:    true,
				expectSecondScopes:   true,
				paramAccessTokens: []auth.Token{
					&mockToken{token: "mock-access-token"},
					&mockToken{token: "mock-access-token"},
				},
			},
			cluster: "cluster/resource/name",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountNamespace("default"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			defaultKubeConfigSA: "lockdown-sa",
			expectedCreds: &auth.RESTConfig{
				Host:        "https://cluster/resource/name",
				BearerToken: "mock-bearer-token",
				CAData:      []byte("ca-data"),
			},
		},
		{
			name: "restconfig from default kubeconfig service account using lockdown support - object level disabled",
			provider: &mockProvider{
				returnName:        "mock-provider",
				returnAccessToken: &mockToken{token: "mock-access-token"},
				returnRESTConfig: &auth.RESTConfig{
					Host:        "https://cluster/resource/name",
					BearerToken: "mock-bearer-token",
					CAData:      []byte("ca-data"),
				},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  *lockdownServiceAccount,
				paramOIDCTokenClient: oidcClient,
				paramCluster:         "cluster/resource/name",
				paramFirstScopes:     []string{"first-token"},
				paramSecondScopes:    []string{"second-token"},
				expectFirstScopes:    true,
				expectSecondScopes:   true,
				paramAccessTokens: []auth.Token{
					&mockToken{token: "mock-access-token"},
					&mockToken{token: "mock-access-token"},
				},
			},
			cluster: "cluster/resource/name",
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountNamespace("default"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			defaultKubeConfigSA: "lockdown-sa",
			disableObjectLevel:  true,
			expectedCreds: &auth.RESTConfig{
				Host:        "https://cluster/resource/name",
				BearerToken: "mock-bearer-token",
				CAData:      []byte("ca-data"),
			},
			expectedErr: "ObjectLevelWorkloadIdentity feature gate is not enabled",
		},
		{
			name: "disable object level workload identity",
			provider: &mockProvider{
				paramServiceAccount: *defaultServiceAccount,
				paramFirstScopes:    []string{"first-token"},
				paramSecondScopes:   []string{"second-token"},
				expectFirstScopes:   true,
				expectSecondScopes:  true,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(saRef.Name),
				auth.WithServiceAccountNamespace(saRef.Namespace),
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
				auth.EnableObjectLevelWorkloadIdentity()
				t.Cleanup(auth.DisableObjectLevelWorkloadIdentity)
			}

			if tt.defaultKubeConfigSA != "" {
				auth.SetDefaultKubeConfigServiceAccount(tt.defaultKubeConfigSA)
				t.Cleanup(func() { auth.SetDefaultKubeConfigServiceAccount("") })
			}

			if tt.cluster != "" {
				tt.opts = append(tt.opts, auth.WithClusterResource(tt.cluster))
			}

			creds, err := auth.GetRESTConfig(ctx, tt.provider, tt.opts...)

			if tt.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectedErr))
				g.Expect(creds).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(creds).To(Equal(tt.expectedCreds))
			}
		})
	}
}
