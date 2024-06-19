/*
Copyright 2023 The Flux authors

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

package git

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	"github.com/fluxcd/pkg/auth/github"
	"github.com/fluxcd/pkg/auth/internal/testutils"
)

func TestGetCredentials(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour)
	auth.InitCache(testutils.NewDummyCache())
	customCache := testutils.NewDummyCache()

	tests := []struct {
		name            string
		authOpts        *auth.AuthOptions
		provider        string
		responseBody    string
		beforeFunc      func(t *WithT, authOpts *auth.AuthOptions, serverURL string)
		afterFunc       func(t *WithT, cache auth.Store, creds Credentials)
		expectCacheHit  bool
		wantCredentials *Credentials
	}{
		{
			name:     "get credentials from github",
			provider: auth.ProviderGitHub,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key: "github-123",
				},
			},
			responseBody: `{
	"token": "access-token",
	"expires_at": "2029-11-10T23:00:00Z"
}`,
			beforeFunc: func(t *WithT, authOpts *auth.AuthOptions, serverURL string) {
				pk, err := createPrivateKey()
				t.Expect(err).ToNot(HaveOccurred())
				authOpts.Secret = &corev1.Secret{
					Data: map[string][]byte{
						github.ApiURLKey:            []byte(serverURL),
						github.AppIDKey:             []byte("127"),
						github.AppInstallationIDKey: []byte("300"),
						github.AppPkKey:             pk,
					},
				}
			},
			afterFunc: func(t *WithT, cache auth.Store, creds Credentials) {
				val, ok := cache.Get("github-123")
				t.Expect(ok).To(BeTrue())
				credentials := val.(Credentials)
				t.Expect(credentials).To(Equal(creds))
			},
			wantCredentials: &Credentials{
				Username: GitHubAccessTokenUsername,
				Password: "access-token",
			},
		},
		{
			name:     "get credentials from cache",
			provider: auth.ProviderGitHub,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key: "github-123",
				},
			},
			expectCacheHit: true,
			wantCredentials: &Credentials{
				Username: GitHubAccessTokenUsername,
				Password: "access-token",
			},
		},
		{
			name:     "get credentials from github with local cache",
			provider: auth.ProviderGitHub,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key:   "github-local-123",
					Cache: customCache,
				},
			},
			responseBody: `{
	"token": "access-token",
	"expires_at": "2029-11-10T23:00:00Z"
}`,
			beforeFunc: func(t *WithT, authOpts *auth.AuthOptions, serverURL string) {
				pk, err := createPrivateKey()
				t.Expect(err).ToNot(HaveOccurred())
				authOpts.Secret = &corev1.Secret{
					Data: map[string][]byte{
						github.ApiURLKey:            []byte(serverURL),
						github.AppIDKey:             []byte("127"),
						github.AppInstallationIDKey: []byte("300"),
						github.AppPkKey:             pk,
					},
				}
			},
			afterFunc: func(t *WithT, cache auth.Store, creds Credentials) {
				val, ok := cache.Get("github-local-123")
				t.Expect(ok).To(BeTrue())
				credentials := val.(Credentials)
				t.Expect(credentials).To(Equal(creds))
			},
			wantCredentials: &Credentials{
				Username: GitHubAccessTokenUsername,
				Password: "access-token",
			},
		},
		{
			name:     "get credentials from local cache",
			provider: auth.ProviderGitHub,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key:   "github-local-123",
					Cache: customCache,
				},
			},
			expectCacheHit: true,
			wantCredentials: &Credentials{
				Username: GitHubAccessTokenUsername,
				Password: "access-token",
			},
		},
		{
			name:     "get credentials from github if cache key is absent",
			provider: auth.ProviderGitHub,
			responseBody: `{
	"token": "access-token",
	"expires_at": "2029-11-10T23:00:00Z"
}`,
			authOpts: &auth.AuthOptions{},
			beforeFunc: func(t *WithT, authOpts *auth.AuthOptions, serverURL string) {
				pk, err := createPrivateKey()
				t.Expect(err).ToNot(HaveOccurred())
				authOpts.Secret = &corev1.Secret{
					Data: map[string][]byte{
						github.ApiURLKey:            []byte(serverURL),
						github.AppIDKey:             []byte("127"),
						github.AppInstallationIDKey: []byte("300"),
						github.AppPkKey:             pk,
					},
				}
			},
			wantCredentials: &Credentials{
				Username: GitHubAccessTokenUsername,
				Password: "access-token",
			},
		},
		{
			name:     "get credentials from azure",
			provider: auth.ProviderAzure,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key: "azure-123",
				},
				ProviderOptions: auth.ProviderOptions{
					AzureOpts: []azure.ProviderOptFunc{
						azure.WithCredential(&azure.FakeTokenCredential{
							Token:     "devops-token",
							ExpiresOn: expiresAt,
						}),
					},
				},
			},
			afterFunc: func(t *WithT, cache auth.Store, creds Credentials) {
				val, ok := cache.Get("azure-123")
				t.Expect(ok).To(BeTrue())
				credentials := val.(Credentials)
				t.Expect(credentials).To(Equal(creds))
			},
			wantCredentials: &Credentials{
				BearerToken: "devops-token",
			},
		},
		{
			name:     "get credentials from gcp",
			provider: auth.ProviderGCP,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key: "gcp-123",
				},
			},
			responseBody: `{
	"access_token": "access-token",
	"expires_in": 10,
	"token_type": "Bearer"
}`,
			beforeFunc: func(_ *WithT, authOpts *auth.AuthOptions, serverURL string) {
				authOpts.ProviderOptions.GcpOpts = []gcp.ProviderOptFunc{
					gcp.WithTokenURL(serverURL),
					gcp.WithEmailURL(fmt.Sprintf("%s/email", serverURL)),
				}
			},
			afterFunc: func(t *WithT, cache auth.Store, creds Credentials) {
				val, ok := cache.Get("gcp-123")
				t.Expect(ok).To(BeTrue())
				credentials := val.(Credentials)
				t.Expect(credentials).To(Equal(creds))
			},
			wantCredentials: &Credentials{
				Username: "git@fluxcd.com",
				Password: "access-token",
			},
		},
		{
			name:            "unknown provider",
			provider:        "generic",
			wantCredentials: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var count int
			handler := func(w http.ResponseWriter, r *http.Request) {
				count += 1
				// this is required for the GCP provider to be able to fetch
				// the Service Account email.
				if tt.provider == auth.ProviderGCP && strings.HasSuffix(r.URL.Path, "email") {
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(tt.wantCredentials.Username))
					return
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.responseBody))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			if tt.beforeFunc != nil {
				tt.beforeFunc(g, tt.authOpts, srv.URL)
			}

			ctx := context.WithValue(context.TODO(), "scope", pointer.String(""))
			creds, err := GetCredentials(ctx, tt.provider, tt.authOpts)
			g.Expect(err).ToNot(HaveOccurred())

			if tt.wantCredentials == nil {
				g.Expect(creds).To(BeNil())
				return
			}
			g.Expect(*creds).To(Equal(*tt.wantCredentials))

			if tt.afterFunc != nil {
				tt.afterFunc(g, tt.authOpts.GetCache(), *creds)
			}

			if tt.expectCacheHit {
				g.Expect(count).To(Equal(0))
			} else {
				// For Azure, the token is returned through a static object, so verify
				// that we didn't hit the cache by asserting that the scope was embedded
				// inside the context as that is the behavior of FakeTokenCredential.
				if tt.provider == auth.ProviderAzure {
					val := ctx.Value("scope").(*string)
					g.Expect(*val).ToNot(BeEmpty())
				} else {
					g.Expect(count).To(BeNumerically(">", 0))
				}
			}
		})
	}
}

func createPrivateKey() ([]byte, error) {
	privatekey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	var privateKeyBytes []byte = x509.MarshalPKCS1PrivateKey(privatekey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	pk := pem.EncodeToMemory(privateKeyBlock)
	return pk, nil
}
