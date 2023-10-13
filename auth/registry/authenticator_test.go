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

package registry

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	"github.com/fluxcd/pkg/auth/internal/testutils"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-containerregistry/pkg/authn"

	. "github.com/onsi/gomega"
)

func TestGetAuthenticator(t *testing.T) {
	g := NewWithT(t)
	expiresAt := time.Now().UTC().Add(time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.RegisteredClaims{
		Issuer:    "auth.microsoft.com",
		Subject:   "fluxcd",
		ExpiresAt: jwt.NewNumericDate(expiresAt),
	})

	pk, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())
	tokenStr, err := token.SignedString(pk)
	g.Expect(err).ToNot(HaveOccurred())

	auth.InitCache(testutils.NewDummyCache())
	customCache := testutils.NewDummyCache()

	tests := []struct {
		name           string
		authOpts       *auth.AuthOptions
		provider       string
		responseBody   string
		beforeFunc     func(authOpts *auth.AuthOptions, serverURL string, registry *string)
		afterFunc      func(t *WithT, cache auth.Store, authConfig authn.AuthConfig)
		expectCacheHit bool
		wantAuthConfig *authn.AuthConfig
	}{
		{
			name:     "get authenticator from gcp",
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
			beforeFunc: func(authOpts *auth.AuthOptions, serverURL string, registry *string) {
				authOpts.ProviderOptions.GcpOpts = []gcp.ProviderOptFunc{gcp.WithTokenURL(serverURL), gcp.WithEmailURL(serverURL)}
			},
			wantAuthConfig: &authn.AuthConfig{
				Username: gcp.DefaultGARUsername,
				Password: "access-token",
			},
			afterFunc: func(t *WithT, cache auth.Store, authConfig authn.AuthConfig) {
				val, ok := cache.Get("gcp-123")
				t.Expect(ok).To(BeTrue())
				ac := val.(authn.AuthConfig)
				t.Expect(ac).To(Equal(authConfig))
			},
		},
		{
			name:     "get authenticator from global cache",
			provider: auth.ProviderGCP,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key: "gcp-123",
				},
			},
			expectCacheHit: true,
			wantAuthConfig: &authn.AuthConfig{
				Username: gcp.DefaultGARUsername,
				Password: "access-token",
			},
		},
		{
			name:     "get authenticator from gcp with local cache",
			provider: auth.ProviderGCP,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key:   "gcp-local-123",
					Cache: customCache,
				},
			},
			responseBody: `{
	"access_token": "access-token",
	"expires_in": 10,
	"token_type": "Bearer"
}`,
			beforeFunc: func(authOpts *auth.AuthOptions, serverURL string, registry *string) {
				authOpts.ProviderOptions.GcpOpts = []gcp.ProviderOptFunc{gcp.WithTokenURL(serverURL), gcp.WithEmailURL(serverURL)}
			},
			wantAuthConfig: &authn.AuthConfig{
				Username: gcp.DefaultGARUsername,
				Password: "access-token",
			},
			afterFunc: func(t *WithT, cache auth.Store, authConfig authn.AuthConfig) {
				val, ok := cache.Get("gcp-local-123")
				t.Expect(ok).To(BeTrue())
				ac := val.(authn.AuthConfig)
				t.Expect(ac).To(Equal(authConfig))
			},
		},
		{
			name:     "get authenticator from global cache",
			provider: auth.ProviderGCP,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key:   "gcp-local-123",
					Cache: customCache,
				},
			},
			expectCacheHit: true,
			wantAuthConfig: &authn.AuthConfig{
				Username: gcp.DefaultGARUsername,
				Password: "access-token",
			},
		},
		{
			name:     "get authenticator from gcp if cache key is absent",
			provider: auth.ProviderGCP,
			responseBody: `{
	"access_token": "access-token",
	"expires_in": 10,
	"token_type": "Bearer"
}`,
			authOpts: &auth.AuthOptions{},
			beforeFunc: func(authOpts *auth.AuthOptions, serverURL string, _ *string) {
				authOpts.ProviderOptions.GcpOpts = []gcp.ProviderOptFunc{gcp.WithTokenURL(serverURL), gcp.WithEmailURL(serverURL)}
			},
			wantAuthConfig: &authn.AuthConfig{
				Username: gcp.DefaultGARUsername,
				Password: "access-token",
			},
		},
		{
			name:     "get authenticator from aws",
			provider: auth.ProviderAWS,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key: "aws-123",
				},
			},
			responseBody: fmt.Sprintf(`{
	"authorizationData": [
		{
			"authorizationToken": "c29tZS1rZXk6c29tZS1zZWNyZXQ=",
			"expiresAt": %d
		}
	]
}`, expiresAt.Unix()),
			beforeFunc: func(authOpts *auth.AuthOptions, serverURL string, registry *string) {
				cfg := awssdk.NewConfig()
				cfg.EndpointResolverWithOptions = awssdk.EndpointResolverWithOptionsFunc(
					func(service, region string, options ...interface{}) (awssdk.Endpoint, error) {
						return awssdk.Endpoint{URL: serverURL}, nil
					})
				cfg.Credentials = credentials.NewStaticCredentialsProvider("x", "y", "z")
				authOpts.ProviderOptions.AwsOpts = []aws.ProviderOptFunc{aws.WithConfig(*cfg)}
				*registry = "0123.dkr.ecr.us-east-1.amazonaws.com"
			},
			wantAuthConfig: &authn.AuthConfig{
				Username: "some-key",
				Password: "some-secret",
			},
			afterFunc: func(t *WithT, cache auth.Store, authConfig authn.AuthConfig) {
				val, ok := cache.Get("aws-123")
				t.Expect(ok).To(BeTrue())
				ac := val.(authn.AuthConfig)
				t.Expect(ac).To(Equal(authConfig))
			},
		},
		{
			name:     "get authenticator from azure",
			provider: auth.ProviderAzure,
			authOpts: &auth.AuthOptions{
				CacheOptions: auth.CacheOptions{
					Key: "azure-123",
				},
			},
			responseBody: fmt.Sprintf(`{"refresh_token": "%s"}`, tokenStr),
			beforeFunc: func(authOpts *auth.AuthOptions, serverURL string, registry *string) {
				authOpts.ProviderOptions.AzureOpts = []azure.ProviderOptFunc{
					azure.WithCredential(&azure.FakeTokenCredential{Token: "foo"}),
				}
				*registry = serverURL
			},
			wantAuthConfig: &authn.AuthConfig{
				Username: "00000000-0000-0000-0000-000000000000",
				Password: tokenStr,
			},
			afterFunc: func(t *WithT, cache auth.Store, authConfig authn.AuthConfig) {
				val, ok := cache.Get("azure-123")
				t.Expect(ok).To(BeTrue())
				ac := val.(authn.AuthConfig)
				t.Expect(ac).To(Equal(authConfig))
			},
		},
		{
			name:           "unknown provider",
			provider:       "generic",
			wantAuthConfig: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var count int
			handler := func(w http.ResponseWriter, r *http.Request) {
				count += 1
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.responseBody))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			var registry string
			if tt.beforeFunc != nil {
				tt.beforeFunc(tt.authOpts, srv.URL, &registry)
			}

			authenticator, err := GetAuthenticator(context.TODO(), registry, tt.provider, tt.authOpts)
			g.Expect(err).ToNot(HaveOccurred())
			if tt.wantAuthConfig == nil {
				g.Expect(authenticator).To(BeNil())
				return
			}
			ac, err := authenticator.Authorization()
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(*ac).To(Equal(*tt.wantAuthConfig))
			if tt.afterFunc != nil {
				tt.afterFunc(g, tt.authOpts.GetCache(), *ac)
			}
			if tt.expectCacheHit {
				g.Expect(count).To(Equal(0))
			} else {
				g.Expect(count).To(BeNumerically(">", 0))
			}
		})
	}
}
