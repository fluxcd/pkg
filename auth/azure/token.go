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
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// Token is the Azure token.
type Token struct{ azcore.AccessToken }

type staticTokenCredential struct{ azcore.AccessToken }

// GetDuration implements auth.Token.
func (t *Token) GetDuration() time.Duration {
	return time.Until(t.ExpiresOn)
}

// Credential gets a token credential for the token to use with Azure libraries.
func (t *Token) Credential() azcore.TokenCredential {
	return &staticTokenCredential{t.AccessToken}
}

// GetToken implements azcore.TokenCredential.
func (s *staticTokenCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return s.AccessToken, nil
}
