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
package gcp

import (
	"context"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
)

const DefaultGARUsername = "oauth2accesstoken"

// GetGARAuthConfig returns an AuthConfig that contains the credentials
// required to authenticate against GAR to access the provided image.
func (p *Provider) GetGARAuthConfig(ctx context.Context) (authn.AuthConfig, time.Duration, error) {
	var authConfig authn.AuthConfig
	var expiresIn time.Duration

	saToken, err := p.GetServiceAccountToken(ctx)
	if err != nil {
		return authConfig, expiresIn, err
	}

	authConfig = authn.AuthConfig{
		Username: DefaultGARUsername,
		Password: saToken.AccessToken,
	}
	expiresIn = time.Second * time.Duration(saToken.ExpiresIn)

	return authConfig, expiresIn, nil
}
