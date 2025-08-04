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

	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	"github.com/fluxcd/pkg/auth/generic"
)

// ProviderByName looks up the implemented providers by name and type.
func ProviderByName[T any](name string) (T, error) {
	var p any
	var zero T

	switch name {
	case aws.ProviderName:
		p = aws.Provider{}
	case azure.ProviderName:
		p = azure.Provider{}
	case gcp.ProviderName:
		p = gcp.NewProvider()
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
