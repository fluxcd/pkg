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
	"fmt"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	"github.com/fluxcd/pkg/auth/generic"
	"github.com/fluxcd/pkg/auth/kubernetes"
)

// CredentialProviderByName looks up the implemented credential providers by name.
func CredentialProviderByName(name string) (auth.CredentialProvider, error) {
	switch name {
	case kubernetes.ProviderName:
		return kubernetes.Provider{}, nil
	default:
		return nil, fmt.Errorf("credential provider '%s' not implemented", name)
	}
}

// withCredentialProvider resolves the credential provider from the credential
// configuration in the options and appends it to the options.
func withCredentialProvider(opts ...auth.Option) ([]auth.Option, error) {
	var o auth.Options
	o.Apply(opts...)
	if o.Credential == nil || o.CredentialProvider != nil {
		return opts, nil
	}
	provider, err := CredentialProviderByName(string(o.Credential.Provider))
	if err != nil {
		return nil, err
	}
	return append(opts, auth.WithCredentialProvider(provider)), nil
}

// ServiceProviderByName looks up the implemented providers by name and type.
func ServiceProviderByName[T any](name string) (T, error) {
	var p any
	var zero T

	switch name {
	case aws.ProviderName:
		p = aws.Provider{}
	case azure.ProviderName:
		p = azure.Provider{}
	case gcp.ProviderName:
		p = gcp.Provider{}
	case generic.ProviderName:
		p = generic.Provider{}
	default:
		return zero, fmt.Errorf("provider '%s' not implemented", name)
	}

	provider, ok := p.(T)
	if !ok {
		return zero, fmt.Errorf("provider '%s' does not implement the expected interface", name)
	}

	return provider, nil
}
