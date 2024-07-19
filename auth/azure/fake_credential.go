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

package azure

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

// FakeTokenCredential is a fake Azure credential provider.
type FakeTokenCredential struct {
	Token string

	ExpiresOn time.Time
	Err       error
}

var _ azcore.TokenCredential = &FakeTokenCredential{}

func (tc *FakeTokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
	if tc.Err != nil {
		return azcore.AccessToken{}, tc.Err
	}

	// Embed the scope inside the context to verify that the desired scope was
	// specified while fetching the token.
	val, ok := ctx.Value("scope").(*string)
	if ok {
		*val = options.Scopes[0]
	} else {
		return azcore.AccessToken{}, fmt.Errorf("unable to get scope")
	}

	return azcore.AccessToken{Token: tc.Token, ExpiresOn: tc.ExpiresOn}, nil
}
