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

package auth

import (
	"time"
)

// Credential is an interface that represents a credential that can be used to
// authenticate requests for a service provider. The only common method is for
// getting the duration of the credential, because different providers have
// different ways of representing the credential. For example, Azure and GCP use
// a single string, while AWS uses three strings: access key ID, secret access
// key and token session. Consumers of this interface should know what type to
// cast it to.
type Credential interface {
	// GetDuration returns the duration for which the credential will still be
	// valid relative to approximately time.Now(). This is used to determine
	// when the credential should be renewed.
	GetDuration() time.Duration
}

// JWTCredential is a credential represented by a JWT. It is used as the
// identity passed to providers that exchange JWTs for service provider
// credentials, and by providers that use the JWT directly as the access
// credential.
type JWTCredential struct {
	// Provider is the credential provider that minted the JWT, e.g.
	// `kubernetes` or `spiffe`.
	Provider string
	// Token is the serialized JWT.
	Token string
	// ExpiresAt is the expiration time extracted from the JWT.
	ExpiresAt time.Time
}

var _ Credential = (*JWTCredential)(nil)

// GetDuration implements Credential.
func (c *JWTCredential) GetDuration() time.Duration {
	return time.Until(c.ExpiresAt)
}
