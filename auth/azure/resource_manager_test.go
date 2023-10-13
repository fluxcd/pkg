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

package azure

import (
	"context"
	"errors"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	_ "github.com/Azure/azure-sdk-for-go/sdk/azcore/arm/runtime"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

func TestGetResourceManagerToken(t *testing.T) {
	tests := []struct {
		name      string
		tokenCred azcore.TokenCredential
		opts      []ProviderOptFunc
		wantToken string
		wantScope string
		wantErr   error
	}{
		{
			name: "default scope",
			tokenCred: &FakeTokenCredential{
				Token: "foo",
			},
			// https://github.com/Azure/azure-sdk-for-go/blob/dd448cf29c643578b23016ca24bdc2316bd70931/sdk/azcore/arm/runtime/runtime.go#L22
			wantScope: "https://management.azure.com/.default",
			wantToken: "foo",
		},
		{
			name: "custom scope",
			tokenCred: &FakeTokenCredential{
				Token: "foo",
			},
			opts: []ProviderOptFunc{WithAzureGovtScope()},
			// https://github.com/Azure/azure-sdk-for-go/blob/dd448cf29c643578b23016ca24bdc2316bd70931/sdk/azcore/arm/runtime/runtime.go#L18
			wantScope: "https://management.usgovcloudapi.net/.default",
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
			provider := NewProvider(tt.opts...)
			provider.credential = tt.tokenCred
			ctx := context.WithValue(context.TODO(), "scope", pointer.String(""))
			token, err := provider.GetResourceManagerToken(ctx)

			if tt.wantErr != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(Equal(errors.New("oh no!")))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(token.Token).To(Equal(tt.wantToken))
				scope := ctx.Value("scope").(*string)
				g.Expect(*scope).To(Equal(tt.wantScope))
			}
		})
	}
}
