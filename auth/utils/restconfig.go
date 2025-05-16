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

package authutils

import (
	"context"

	"k8s.io/client-go/rest"

	"github.com/fluxcd/pkg/auth"
)

// GetRESTConfig retrieves a restconfig for the given cluster resource
// name and provider.
func GetRESTConfig(ctx context.Context, providerName string,
	cluster string, opts ...auth.Option) (*rest.Config, error) {

	provider, err := ProviderByName(providerName)
	if err != nil {
		return nil, err
	}

	return auth.GetRESTConfig(ctx, provider, cluster, opts...)
}
