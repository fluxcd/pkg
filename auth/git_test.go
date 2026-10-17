/*
Copyright 2026 The Flux authors

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

func TestGetGitCredentials(t *testing.T) {
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

	gitURL, err := url.Parse("https://git.example.com/org/repo.git")
	g.Expect(err).NotTo(HaveOccurred())

	now := time.Now()

	for _, tt := range []struct {
		name          string
		provider      *mockProvider
		gitURL        *url.URL
		opts          []auth.Option
		expectedCreds *auth.GitCredentials
		expectedErr   string
	}{
		{
			name: "git credentials from controller access token",
			provider: &mockProvider{
				returnGitInput:        "git-cache-key",
				returnControllerToken: &mockToken{token: "mock-default-token"},
				returnGitCredentials: &auth.GitCredentials{
					BearerToken: "mock-bearer-token",
				},
				paramGitURL:      gitURL,
				paramAccessToken: &mockToken{token: "mock-default-token"},
			},
			gitURL: gitURL,
			opts: []auth.Option{
				auth.WithCredentialAudiences("audience1", "audience2"),
				auth.WithScopes("scope1", "scope2"),
				auth.WithAWSRegion("us-east-1"),
				auth.WithAWSSTSEndpoint("https://sts.some-cloud.io"),
				auth.WithProxyURL(url.URL{Scheme: "http", Host: "proxy.io:8080"}),
				auth.WithCAData("ca-data"),
			},
			expectedCreds: &auth.GitCredentials{
				BearerToken: "mock-bearer-token",
			},
		},
		{
			name: "git credentials from access token from service account",
			provider: &mockProvider{
				returnName:        "mock-provider",
				returnGitInput:    "git-cache-key",
				returnAccessToken: &mockToken{token: "mock-access-token"},
				returnGitCredentials: &auth.GitCredentials{
					Username: "user",
					Password: "pass",
				},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient: oidcClient,
				paramGitURL:          gitURL,
				paramAccessToken:     &mockToken{token: "mock-access-token"},
			},
			gitURL: gitURL,
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
			expectedCreds: &auth.GitCredentials{
				Username: "user",
				Password: "pass",
			},
		},
		{
			name: "git credentials from access token from explicit service account",
			provider: &mockProvider{
				returnName:        "mock-provider",
				returnGitInput:    "git-cache-key",
				returnAccessToken: &mockToken{token: "mock-access-token"},
				returnGitCredentials: &auth.GitCredentials{
					Username: "user",
					Password: "pass",
				},
				paramAudiences:       []string{"audience1", "audience2"},
				paramServiceAccount:  &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramOIDCTokenClient: oidcClient,
				paramGitURL:          gitURL,
				paramAccessToken:     &mockToken{token: "mock-access-token"},
			},
			gitURL: gitURL,
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
			expectedCreds: &auth.GitCredentials{
				Username: "user",
				Password: "pass",
			},
		},
		{
			name: "all the options are taken into account in the cache key",
			provider: &mockProvider{
				returnName:          "mock-provider",
				returnIdentity:      "mock-identity",
				returnGitInput:      "git-cache-key",
				paramServiceAccount: &kubernetes.ServiceAccountReference{Name: defaultServiceAccount.Name, Namespace: defaultServiceAccount.Namespace},
				paramGitURL:         gitURL,
			},
			gitURL: gitURL,
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
					tokenCache, err := cache.NewTokenCache(2)
					g.Expect(err).NotTo(HaveOccurred())

					const accessTokenKey = "6c016e84cc74ad7bf6bd8770cdb6a2efca3e185fdf90badd985c77984784f5c3"
					var token auth.Token = &mockToken{token: "cached-token"}
					cachedToken, ok, err := tokenCache.GetOrSet(ctx, accessTokenKey, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedToken).To(Equal(token))

					const gitCredentialsKey = "f78ca5404e02a7b139ceb6b925ac52344cb77f33e92197b153b9d87615e6b91b"
					token = &auth.GitCredentials{
						Username:  "cached-user",
						Password:  "cached-pass",
						ExpiresAt: now.Add(time.Hour),
					}
					cachedToken, ok, err = tokenCache.GetOrSet(ctx, gitCredentialsKey, func(ctx context.Context) (cache.Token, error) {
						return token, nil
					})
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(ok).To(BeFalse())
					g.Expect(cachedToken).To(Equal(token))

					o.Cache = tokenCache
				},
			},
			expectedCreds: &auth.GitCredentials{
				Username:  "cached-user",
				Password:  "cached-pass",
				ExpiresAt: now.Add(time.Hour),
			},
		},
		{
			name: "missing Git URL",
			provider: &mockProvider{
				paramGitURL: gitURL,
			},
			expectedErr: "a Git repository URL is required",
		},
		{
			name: "error parsing Git repository",
			provider: &mockProvider{
				paramGitURL:  gitURL,
				returnGitErr: "mock error",
			},
			gitURL:      gitURL,
			expectedErr: "mock error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tt.provider.t = t

			opts := tt.opts
			if tt.gitURL != nil {
				opts = append(opts, auth.WithGitURL(*tt.gitURL))
			}

			creds, err := auth.GetGitCredentials(ctx, tt.provider, opts...)

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
