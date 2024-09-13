/*
Copyright 2024 The Flux authors

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

package azure

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	. "github.com/onsi/gomega"
)

func TestGetProviderToken(t *testing.T) {
	proxy, _ := url.Parse("http://localhost:8080")
	tests := []struct {
		name      string
		tokenCred azcore.TokenCredential
		opts      []OptFunc
		wantToken string
		wantScope string
		wantErr   error
	}{
		{
			name: "azure devops scope",
			tokenCred: &FakeTokenCredential{
				Token: "foo",
			},
			opts:      []OptFunc{WithAzureDevOpsScope()},
			wantScope: AzureDevOpsRestApiScope,
			wantToken: "foo",
		},
		{
			name: "custom scope",
			tokenCred: &FakeTokenCredential{
				Token: "foo",
			},
			opts:      []OptFunc{WithScope([]string{"custom scope"})},
			wantScope: "custom scope",
			wantToken: "foo",
		},
		{
			name: "custom scope and azure devops scope",
			tokenCred: &FakeTokenCredential{
				Token: "foo",
			},
			opts:      []OptFunc{WithScope([]string{"custom scope"}), WithAzureDevOpsScope()},
			wantScope: fmt.Sprintf("custom scope,%s", AzureDevOpsRestApiScope),
			wantToken: "foo",
		},
		{
			name: "no scope specified",
			tokenCred: &FakeTokenCredential{
				Token: "foo",
			},
			wantScope: cloud.AzurePublic.Services[cloud.ResourceManager].Endpoint + "/" + ".default",
			wantToken: "foo",
		},
		{
			name: "with proxy url",
			tokenCred: &FakeTokenCredential{
				Token: "foo",
			},
			opts:      []OptFunc{WithProxyURL(proxy)},
			wantToken: "foo",
		},
		{
			name: "error",
			tokenCred: &FakeTokenCredential{
				Err: errors.New("oh no!"),
			},
			wantErr: errors.New("oh no!"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			tt.opts = append(tt.opts, WithCredential(tt.tokenCred))
			client, err := New(tt.opts...)
			g.Expect(err).ToNot(HaveOccurred())
			str := ""
			ctx := context.WithValue(context.TODO(), "scope", &str)
			token, err := client.GetToken(ctx)

			if tt.wantErr != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(Equal(tt.wantErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(token.Token).To(Equal(tt.wantToken))
				if tt.wantScope != "" {
					scope := ctx.Value("scope").(*string)
					g.Expect(*scope).To(Equal(tt.wantScope))
				}
			}
		})
	}
}
