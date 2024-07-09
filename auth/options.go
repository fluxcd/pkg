/*
Copyright 2024 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License"); you may not use
this file except in compliance with the License.

You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed
under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
CONDITIONS OF ANY KIND, either express or implied. See the License for the
specific language governing permissions and limitations under the License.
*/

package auth

import (
	"github.com/fluxcd/pkg/auth/azure"
)

const (
	ProviderAzure = "azure"
)

// AuthOptions contains options that can be used for authentication.
type AuthOptions struct {
	// ProviderOptions specifies the options to configure various authentication
	// providers.
	ProviderOptions ProviderOptions
}

// ProviderOptions contains options to configure various authentication
// providers.
type ProviderOptions struct {
	AzureOpts []azure.ProviderOptFunc
}
