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
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

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

	now := time.Now()

	for _, tt := range []struct {
		name               string
		provider           *mockProvider
		cluster            string
		opts               []auth.Option
		disableObjectLevel bool
		expectedCreds      *auth.RESTConfig
		expectedErr        string
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
				paramCluster:      "cluster/resource/name",
				paramFirstScopes:  []string{"first-token"},
				paramSecondScopes: []string{"second-token"},
				paramAccessTokens: []auth.Token{
					&mockToken{token: "mock-default-token"},
					&mockToken{token: "mock-default-token"},
				},
			},
			cluster: "cluster/resource/name",
			opts: []auth.Option{
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
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
				returnAudience:    "mock-audience",
				returnAccessToken: &mockToken{token: "mock-access-token"},
				returnRESTConfig: &auth.RESTConfig{
					Host:        "https://cluster/resource/name",
					BearerToken: "mock-bearer-token",
					CAData:      []byte("ca-data"),
				},
				paramServiceAccount:  *defaultServiceAccount,
				paramOIDCTokenClient: oidcClient,
				paramCluster:         "cluster/resource/name",
				paramFirstScopes:     []string{"first-token"},
				paramSecondScopes:    []string{"second-token"},
				paramAccessTokens: []auth.Token{
					&mockToken{token: "mock-access-token"},
					&mockToken{token: "mock-access-token"},
				},
			},
			cluster: "cluster/resource/name",
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
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
				returnAudience:      "mock-audience",
				returnIdentity:      "mock-identity",
				paramServiceAccount: *defaultServiceAccount,
				paramCluster:        "cluster/resource/name",
				paramFirstScopes:    []string{"first-token"},
				paramSecondScopes:   []string{"second-token"},
			},
			cluster: "cluster/resource/name",
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(3)
					g.Expect(err).NotTo(HaveOccurred())

					accessTokenKey := "4be101e2e5adac5ce660b83cea68103cf6f1f9e4bdf162dfd1a712345502be19"
					var token auth.Token = &mockToken{token: "cached-token"}
					cachedToken, ok, err := tokenCache.GetOrSet(ctx, accessTokenKey, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedToken).To(Equal(token))

					accessTokenKey = "b9f570ceefc4521e2da1555d56354e201ffc146afb0de6d409e73aa439ab8062"
					token = &mockToken{token: "cached-token"}
					cachedToken, ok, err = tokenCache.GetOrSet(ctx, accessTokenKey, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedToken).To(Equal(token))

					const restConfigKey = "74204a2b428767b5b6736573baaa78cc420b52e5ce8d39fb2a14237f929a001c"
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
			name: "disable object level workload identity",
			provider: &mockProvider{
				paramServiceAccount: *defaultServiceAccount,
			},
			opts: []auth.Option{
				auth.WithServiceAccount(saRef, kubeClient),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
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

			creds, err := auth.GetRESTConfig(ctx, tt.provider, tt.cluster, tt.opts...)

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
