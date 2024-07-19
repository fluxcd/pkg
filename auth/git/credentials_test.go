/*
Copyright 2024 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License. You may obtain a copy of the
License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package git

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/cache"
)

func TestGetCredentials(t *testing.T) {
	tests := []struct {
		name            string
		url             string
		authOpts        *AuthOptions
		provider        string
		wantCredentials *Credentials
		wantScope       string
	}{
		{
			name:     "get credentials from azure",
			url:      "https://dev.azure.com/foo/bar/_git/baz",
			provider: auth.ProviderAzure,
			authOpts: &AuthOptions{
				ProviderOptions: ProviderOptions{
					AzureOpts: []azure.ProviderOptFunc{
						azure.WithCredential(&azure.FakeTokenCredential{
							Token: "ado-token",
						}),
						azure.WithAzureDevOpsScope(),
					},
				},
			},
			wantCredentials: &Credentials{
				BearerToken: "ado-token",
			},
			wantScope: "499b84ac-1321-427f-aa17-267ca6975798/.default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.WithValue(context.TODO(), "scope", pointer.String(""))
			creds, err := GetCredentials(ctx, tt.url, tt.provider, tt.authOpts)
			g.Expect(err).ToNot(HaveOccurred())

			if tt.wantCredentials != nil {
				g.Expect(*creds).To(Equal(*tt.wantCredentials))
				val := ctx.Value("scope").(*string)
				g.Expect(*val).ToNot(BeEmpty())
				g.Expect(*val).To(Equal((tt.wantScope)))

				data := creds.ToSecretData()
				g.Expect(data).ToNot((BeNil()))
				g.Expect(data).To(HaveKey("bearerToken"))
				credBytes := []byte(tt.wantCredentials.BearerToken)
				g.Expect(data["bearerToken"]).To(Equal(credBytes))
			} else {
				g.Expect(creds).To(BeNil())
			}
		})
	}
}

func TestGetCredentialsWithCache(t *testing.T) {
	expiresOn := time.Now().Add(10 * time.Second)
	tests := []struct {
		name            string
		url             string
		authOpts        *AuthOptions
		provider        string
		wantCredentials *Credentials
		wantScope       string
	}{
		{
			name:     "get credentials from azure",
			url:      "https://dev.azure.com/foo/bar/_git/baz",
			provider: auth.ProviderAzure,
			authOpts: &AuthOptions{
				ProviderOptions: ProviderOptions{
					AzureOpts: []azure.ProviderOptFunc{
						azure.WithCredential(&azure.FakeTokenCredential{
							Token:     "ado-token",
							ExpiresOn: expiresOn,
						}),
						azure.WithAzureDevOpsScope(),
					},
				},
			},
			wantCredentials: &Credentials{
				BearerToken: "ado-token",
			},
			wantScope: "499b84ac-1321-427f-aa17-267ca6975798/.default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			cache, err := cache.New(5, cache.StoreObjectKeyFunc,
				cache.WithCleanupInterval[cache.StoreObject[Credentials]](1*time.Second))
			g.Expect(err).ToNot(HaveOccurred())
			tt.authOpts.Cache = cache

			ctx := context.WithValue(context.TODO(), "scope", pointer.String(""))
			creds, err := GetCredentials(ctx, tt.url, tt.provider, tt.authOpts)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(creds).ToNot(BeNil())
			CacheCredentials(ctx, tt.url, tt.authOpts, creds)
			cachedCreds, exists, err := getObjectFromCache(cache, tt.url)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(exists).To(BeTrue())
			g.Expect(cachedCreds).ToNot(BeNil())
			obj, _, err := cache.GetByKey(tt.url)
			g.Expect(err).ToNot(HaveOccurred())
			expiration, err := cache.GetExpiration(obj)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(expiration).ToNot(BeZero())
			g.Expect(expiration).To(BeTemporally("~", expiresOn, 1*time.Second))
		})
	}
}
