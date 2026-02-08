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
	"fmt"
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

	// Create a lockdown service account for testing lockdown functionality.
	lockdownServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "lockdown-sa",
			Namespace: "default",
		},
	}
	err = kubeClient.Create(ctx, lockdownServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a service account with impersonation annotation.
	impersonationServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "impersonation-sa",
			Namespace: "default",
			Annotations: map[string]string{
				"mock-provider.auth.fluxcd.io/impersonation": "roleArn: arn:aws:iam::123456789012:role/target-role\nuseServiceAccount: true",
			},
		},
	}
	err = kubeClient.Create(ctx, impersonationServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a service account with impersonation annotation (no useServiceAccount, defaults to false without lockdown).
	impersonationNoSAServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "impersonation-no-sa",
			Namespace: "default",
			Annotations: map[string]string{
				"mock-provider.auth.fluxcd.io/impersonation": "roleArn: arn:aws:iam::123456789012:role/target-role",
			},
		},
	}
	err = kubeClient.Create(ctx, impersonationNoSAServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a service account with impersonation annotation with explicit useServiceAccount: false.
	impersonationExplicitNoSAServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "impersonation-explicit-no-sa",
			Namespace: "default",
			Annotations: map[string]string{
				"mock-provider.auth.fluxcd.io/impersonation": "roleArn: arn:aws:iam::123456789012:role/target-role\nuseServiceAccount: false",
			},
		},
	}
	err = kubeClient.Create(ctx, impersonationExplicitNoSAServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a service account with impersonation annotation (useServiceAccount: false)
	// AND a provider identity annotation (which should be rejected).
	impersonationNoSAWithIdentityServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "impersonation-no-sa-with-identity",
			Namespace: "default",
			Annotations: map[string]string{
				"mock-provider.auth.fluxcd.io/impersonation": "roleArn: arn:aws:iam::123456789012:role/target-role\nuseServiceAccount: false",
			},
		},
	}
	err = kubeClient.Create(ctx, impersonationNoSAWithIdentityServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	// Create a service account with invalid impersonation annotation.
	invalidImpersonationServiceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "invalid-impersonation-sa",
			Namespace: "default",
			Annotations: map[string]string{
				"mock-provider.auth.fluxcd.io/impersonation": "{{invalid yaml",
			},
		},
	}
	err = kubeClient.Create(ctx, invalidImpersonationServiceAccount)
	g.Expect(err).NotTo(HaveOccurred())

	for _, tt := range []struct {
		name               string
		provider           *mockProvider
		withImpersonation  bool
		opts               []auth.Option
		disableObjectLevel bool
		expectedToken      auth.Token
		expectedErr        string
		verifyIdentity     string
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
			name: "access token from service account using default - for lockdown support",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  *lockdownServiceAccount,
				paramOIDCTokenClient: oidcClient,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountNamespace("default"),
				auth.WithDefaultServiceAccount("lockdown-sa"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedToken: &mockToken{token: "mock-access-token"},
		},
		{
			name: "access token from service account using default - for lockdown support, object level disabled",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  *lockdownServiceAccount,
				paramOIDCTokenClient: oidcClient,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountNamespace("default"),
				auth.WithDefaultServiceAccount("lockdown-sa"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			disableObjectLevel: true,
			expectedToken:      &mockToken{token: "mock-access-token"},
			expectedErr:        "ObjectLevelWorkloadIdentity feature gate is not enabled",
		},
		{
			name: "error when default service account does not exist - for lockdown support",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  *lockdownServiceAccount,
				paramOIDCTokenClient: oidcClient,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountNamespace("default"),
				auth.WithDefaultServiceAccount("non-existent-sa"),
				auth.WithAudiences("audience1", "audience2"),
			},
			expectedErr: "the specified default service account does not exist in the object namespace",
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
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(saRef.Name),
				auth.WithServiceAccountNamespace(saRef.Namespace),
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
			name: "access token from service account with explicit name ignoring default",
			provider: &mockProvider{
				returnName:           "mock-provider",
				returnAccessToken:    &mockToken{token: "mock-access-token"},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  *defaultServiceAccount,
				paramOIDCTokenClient: oidcClient,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(saRef.Name),
				auth.WithServiceAccountNamespace(saRef.Namespace),
				auth.WithDefaultServiceAccount("non-existent-sa"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
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
				paramServiceAccount:  *defaultServiceAccount,
				paramOIDCTokenClient: oidcClient,
			},
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(saRef.Name),
				auth.WithServiceAccountNamespace(saRef.Namespace),
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
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName(saRef.Name),
				auth.WithServiceAccountNamespace(saRef.Namespace),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())

					const key = "6fbdfd364d87e47e6aad554232b927805c949ac461c43eb1c84d7dbcd58c38fb"
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
			expectedErr: "failed to get provider identity from service account 'default/default' annotations:",
		},
		{
			name: "error getting identity using cache",
			provider: &mockProvider{
				returnIdentityErr:   "mock error",
				paramServiceAccount: *defaultServiceAccount,
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
				func(o *auth.Options) {
					tokenCache, err := cache.NewTokenCache(1)
					g.Expect(err).NotTo(HaveOccurred())
					o.Cache = tokenCache
				},
			},
			expectedErr: "failed to get provider identity from service account 'default/default' annotations:",
		},
		{
			name: "disable object level workload identity",
			provider: &mockProvider{
				paramServiceAccount: *defaultServiceAccount,
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
		{
			name: "impersonation skipped without service account",
			provider: &mockProvider{
				returnControllerToken:   &mockToken{token: "mock-controller-token"},
				returnImpersonatedToken: &mockToken{token: "should-not-appear"},
			},
			withImpersonation: true,
			opts: []auth.Option{
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedToken: &mockToken{token: "mock-controller-token"},
		},
		{
			name: "access token from service account with impersonation",
			provider: &mockProvider{
				returnName:                     "mock-provider",
				returnAccessToken:              &mockToken{token: "mock-access-token"},
				returnIdentityForImpersonation: mockIdentity("arn:aws:iam::123456789012:role/target-role"),
				returnImpersonatedToken:        &mockToken{token: "mock-impersonated-sa-token"},
				paramAudiences:                 []string{"audience1", "audience2"},
				paramServiceAccount:            *impersonationServiceAccount,
				paramOIDCTokenClient:           oidcClient,
			},
			withImpersonation: true,
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName("impersonation-sa"),
				auth.WithServiceAccountNamespace("default"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedToken:  &mockToken{token: "mock-impersonated-sa-token"},
			verifyIdentity: "arn:aws:iam::123456789012:role/target-role",
		},
		{
			name: "impersonation with useServiceAccount false uses controller token",
			provider: &mockProvider{
				returnName:                     "mock-provider",
				returnControllerToken:          &mockToken{token: "mock-controller-token"},
				returnIdentityForImpersonation: mockIdentity("arn:aws:iam::123456789012:role/target-role"),
				returnImpersonatedToken:        &mockToken{token: "mock-impersonated-controller-token"},
				paramServiceAccount:            *impersonationNoSAServiceAccount,
			},
			withImpersonation: true,
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName("impersonation-no-sa"),
				auth.WithServiceAccountNamespace("default"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedToken:  &mockToken{token: "mock-impersonated-controller-token"},
			verifyIdentity: "arn:aws:iam::123456789012:role/target-role",
		},
		{
			name: "impersonation with useServiceAccount false and lockdown enabled errors",
			provider: &mockProvider{
				returnName:                     "mock-provider",
				returnIdentityForImpersonation: mockIdentity("arn:aws:iam::123456789012:role/target-role"),
				paramServiceAccount:            *impersonationExplicitNoSAServiceAccount,
			},
			withImpersonation: true,
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountNamespace("default"),
				auth.WithDefaultServiceAccount("impersonation-explicit-no-sa"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedErr: "multi-tenancy lockdown is enabled, impersonation without service account is not allowed",
		},
		{
			name: "impersonation NewTokenForNativeToken error",
			provider: &mockProvider{
				returnName:                     "mock-provider",
				returnAccessToken:              &mockToken{token: "mock-access-token"},
				returnIdentityForImpersonation: mockIdentity("arn:aws:iam::123456789012:role/target-role"),
				returnImpersonateErr:           fmt.Errorf("impersonation failed"),
				paramAudiences:                 []string{"audience1", "audience2"},
				paramServiceAccount:            *impersonationServiceAccount,
				paramOIDCTokenClient:           oidcClient,
			},
			withImpersonation: true,
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName("impersonation-sa"),
				auth.WithServiceAccountNamespace("default"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedErr:    "impersonation failed",
			verifyIdentity: "arn:aws:iam::123456789012:role/target-role",
		},
		{
			name: "impersonation skipped when no impersonation annotation",
			provider: &mockProvider{
				returnName:              "mock-provider",
				returnAccessToken:       &mockToken{token: "mock-access-token"},
				returnImpersonatedToken: &mockToken{token: "should-not-appear"},
				paramAudiences:          []string{"audience1", "audience2"},
				paramServiceAccount:     *defaultServiceAccount,
				paramOIDCTokenClient:    oidcClient,
			},
			withImpersonation: true,
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
			expectedToken: &mockToken{token: "mock-access-token"},
		},
		{
			name: "GetIdentityForImpersonation error",
			provider: &mockProvider{
				returnName:                        "mock-provider",
				returnAccessToken:                 &mockToken{token: "mock-access-token"},
				returnIdentityForImpersonationErr: "impersonation identity lookup failed",
				paramAudiences:                    []string{"audience1", "audience2"},
				paramServiceAccount:               *impersonationServiceAccount,
				paramOIDCTokenClient:              oidcClient,
			},
			withImpersonation: true,
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName("impersonation-sa"),
				auth.WithServiceAccountNamespace("default"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedErr: "failed to get provider identity for impersonation from service account 'default/impersonation-sa'",
		},
		{
			name: "invalid impersonation annotation YAML",
			provider: &mockProvider{
				returnName:          "mock-provider",
				paramServiceAccount: *invalidImpersonationServiceAccount,
			},
			withImpersonation: true,
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName("invalid-impersonation-sa"),
				auth.WithServiceAccountNamespace("default"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedErr: "failed to parse impersonation annotation",
		},
		{
			name: "impersonation useServiceAccount false with identity annotation errors",
			provider: &mockProvider{
				returnName:                     "mock-provider",
				returnIdentity:                 "mock-identity",
				returnIdentityForImpersonation: mockIdentity("arn:aws:iam::123456789012:role/target-role"),
				paramServiceAccount:            *impersonationNoSAWithIdentityServiceAccount,
			},
			withImpersonation: true,
			opts: []auth.Option{
				auth.WithClient(kubeClient),
				auth.WithServiceAccountName("impersonation-no-sa-with-identity"),
				auth.WithServiceAccountNamespace("default"),
				auth.WithAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithSTSRegion("us-east-1"),
				auth.WithSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedErr: "identity annotation is present but the ServiceAccount is not used",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tt.provider.t = t

			if !tt.disableObjectLevel {
				auth.EnableObjectLevelWorkloadIdentity()
				t.Cleanup(auth.DisableObjectLevelWorkloadIdentity)
			}

			var p auth.Provider = tt.provider
			if tt.withImpersonation {
				p = &mockProviderWithImpersonation{mockProvider: tt.provider}
			}

			token, err := auth.GetAccessToken(ctx, p, tt.opts...)

			if tt.expectedErr != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tt.expectedErr)))
				g.Expect(token).To(BeNil())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(token).To(Equal(tt.expectedToken))
			}

			if tt.verifyIdentity != "" {
				g.Expect(tt.provider.gotIdentity).NotTo(BeNil())
				g.Expect(tt.provider.gotIdentity.String()).To(Equal(tt.verifyIdentity))
			}
		})
	}
}

func TestGetAccessToken_ProviderDoesNotSupportOIDCImpersonation(t *testing.T) {
	g := NewWithT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	kubeClient, _ := newTestEnv(t, ctx)

	auth.EnableObjectLevelWorkloadIdentity()
	t.Cleanup(auth.DisableObjectLevelWorkloadIdentity)

	// Create a service account.
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "oidc-test-sa",
			Namespace: "default",
		},
	}
	err := kubeClient.Create(ctx, sa)
	g.Expect(err).NotTo(HaveOccurred())

	// Use a basic provider that only implements auth.Provider, not ProviderWithOIDCImpersonation.
	provider := &mockBasicProvider{returnName: "basic-provider"}

	token, err := auth.GetAccessToken(ctx, provider,
		auth.WithClient(kubeClient),
		auth.WithServiceAccountName("oidc-test-sa"),
		auth.WithServiceAccountNamespace("default"),
	)
	g.Expect(err).To(MatchError(ContainSubstring("provider 'basic-provider' does not support impersonation with OIDC tokens")))
	g.Expect(token).To(BeNil())
}

func TestGetAccessToken_ProviderDoesNotSupportNativeImpersonation(t *testing.T) {
	g := NewWithT(t)

	// Use a basic provider that only implements auth.Provider, not ProviderWithImpersonation.
	provider := &mockBasicProvider{
		returnName:            "basic-provider",
		returnControllerToken: &mockToken{token: "controller-token"},
	}

	token, err := auth.GetAccessToken(context.Background(), provider,
		auth.WithIdentityForImpersonation(mockIdentity("some-target-identity")),
	)
	g.Expect(err).To(MatchError(ContainSubstring("provider 'basic-provider' does not support impersonation with native tokens")))
	g.Expect(token).To(BeNil())
}

func TestGetAccessToken_WithIdentityForImpersonation(t *testing.T) {
	g := NewWithT(t)

	provider := &mockProvider{
		returnControllerToken:   &mockToken{token: "mock-controller-token"},
		returnImpersonatedToken: &mockToken{token: "mock-impersonated-token"},
	}
	provider.t = t

	p := &mockProviderWithImpersonation{mockProvider: provider}

	token, err := auth.GetAccessToken(context.Background(), p,
		auth.WithIdentityForImpersonation(mockIdentity("target-role-arn")),
		auth.WithAudiences("audience1", "audience2"),
		auth.WithScopes("scope1", "scope2"),
		auth.WithSTSRegion("us-east-1"),
		auth.WithSTSEndpoint("https://sts.some-cloud.io"),
		auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
		auth.WithCAData("ca-data"),
	)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(token).To(Equal(&mockToken{token: "mock-impersonated-token"}))
	g.Expect(provider.gotIdentity).NotTo(BeNil())
	g.Expect(provider.gotIdentity.String()).To(Equal("target-role-arn"))
}
