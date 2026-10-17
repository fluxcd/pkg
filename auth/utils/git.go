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
)

// GetGitCredentials looks up the implemented providers that support Git
// and returns the credentials for the specified provider.
//
// When object-level workload identity is configured through the options, the
// ServiceAccount is fetched and validated with the
// github.com/fluxcd/pkg/apis/crypto package before the credentials are
// retrieved.
func GetGitCredentials(ctx context.Context, providerName string,
	opts ...Option) (*auth.GitCredentials, error) {

	var o options
	for _, opt := range opts {
		opt(&o)
	}

	provider, err := ProviderByName[auth.GitCredentialsProvider](providerName)
	if err != nil {
		return nil, fmt.Errorf("provider '%s' does not support Git credentials", providerName)
	}

	saOpts, err := resolveServiceAccount(ctx, providerName, o)
	if err != nil {
		return nil, err
	}
	authOpts := append(slices.Clone(o.authOpts), saOpts...)

	return auth.GetGitCredentials(ctx, provider, authOpts...)
}
