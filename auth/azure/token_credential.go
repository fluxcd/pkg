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

package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"

	"github.com/fluxcd/pkg/auth"
)

type tokenCredential struct {
	ctx  context.Context
	opts []auth.Option
}

// NewTokenCredential creates a new token credential for the given options.
func NewTokenCredential(ctx context.Context, opts ...auth.Option) azcore.TokenCredential {
	return &tokenCredential{ctx, opts}
}

// GetToken implements exported.TokenCredential.
// The context is ignored, use the constructor to set the context.
// This is because some callers of the library pass context.Background()
// when calling this method (e.g. SOPS), so to ensure we have a real
// context we pass it in the constructor.
func (t *tokenCredential) GetToken(_ context.Context, tokenOpts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	opts := t.opts
	if tokenOpts.Scopes != nil {
		opts = append(opts, auth.WithScopes(tokenOpts.Scopes...))
	}
	token, err := auth.GetToken(t.ctx, Provider{}, opts...)
	if err != nil {
		return azcore.AccessToken{}, err
	}
	azureToken, ok := token.(*Token)
	if !ok {
		return azcore.AccessToken{}, fmt.Errorf("failed to cast token to Azure token: %T", token)
	}
	return azureToken.AccessToken, nil
}
