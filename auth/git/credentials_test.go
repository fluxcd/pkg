/*
Copyright 2023 The Flux authors

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
)

func TestGetCredentials(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour)

	tests := []struct {
		name            string
		authOpts        *auth.AuthOptions
		provider        string
		wantCredentials *Credentials
		wantScope       string
	}{
		{
			name:     "get credentials from azure",
			provider: auth.ProviderAzure,
			authOpts: &auth.AuthOptions{
				ProviderOptions: auth.ProviderOptions{
					AzureOpts: []azure.ProviderOptFunc{
						azure.WithCredential(&azure.FakeTokenCredential{
							Token:     "ado-token",
							ExpiresOn: expiresAt,
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
			creds, err := GetCredentials(ctx, tt.provider, tt.authOpts)
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
