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

package utils

import (
	"context"
	"fmt"
	"slices"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/azure"
)

// GitCredentials contains authentication data needed in order to access a Git
// repository.
type GitCredentials struct {
	BearerToken string
	Username    string
	Password    string
}

// GetGitCredentials looks up by the implemented providers that support Git
// and returns the credentials for the provider.
func GetGitCredentials(ctx context.Context, providerName string, opts ...auth.Option) (*GitCredentials, error) {
	opts = slices.Clone(opts)
	switch providerName {
	case azure.ProviderName:
		opts = append(opts, auth.WithScopes(azure.ScopeDevOps))
		token, err := auth.GetAccessToken(ctx, azure.Provider{}, opts...)
		if err != nil {
			return nil, err
		}
		return &GitCredentials{
			BearerToken: token.(*azure.Token).Token,
		}, nil
	default:
		return nil, fmt.Errorf("provider '%s' does not support Git credentials", providerName)
	}
}
