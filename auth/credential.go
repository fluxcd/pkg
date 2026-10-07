/*
Copyright 2026 The Flux authors

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
	"context"
	"time"

	"github.com/fluxcd/pkg/apis/crypto"
)

// Credential is short-lived cryptographic material that can be used to
// authenticate with a remote service. The only common method is for getting the
// duration of the credential, because different credentials have different
// representations. For example, a JWT is a single string, while AWS credentials
// are three strings: access key ID, secret access key and session token.
// Consumers of this interface should know what type to cast it to.
type Credential interface {
	// GetDuration returns the duration for which the credential will still be
	// valid relative to approximately time.Now(). This is used to determine
	// when the credential should be renewed.
	GetDuration() time.Duration
}

// CredentialMaterial is short-lived cryptographic material that can be used to
// authenticate with a remote service, either directly or after being
// exchanged for another credential by a ServiceProvider.
type CredentialMaterial interface {
	Credential

	// Type returns the type of the credential material.
	Type() crypto.CredentialType
}

// JWT is a JSON Web Token credential.
type JWT struct {
	// Token is the JWT string.
	Token string

	// ExpiresAt is the time at which the token expires.
	ExpiresAt time.Time
}

// GetDuration implements Credential.
func (j *JWT) GetDuration() time.Duration {
	return time.Until(j.ExpiresAt)
}

// Type implements CredentialMaterial.
func (j *JWT) Type() crypto.CredentialType {
	return crypto.CredentialTypeJWT
}

// X509 is an X.509 certificate credential.
type X509 struct {
	// Certificate is the PEM-encoded certificate chain.
	Certificate []byte

	// PrivateKey is the PEM-encoded private key.
	PrivateKey []byte

	// ExpiresAt is the time at which the certificate expires.
	ExpiresAt time.Time
}

// GetDuration implements Credential.
func (x *X509) GetDuration() time.Duration {
	return time.Until(x.ExpiresAt)
}

// Type implements CredentialMaterial.
func (x *X509) Type() crypto.CredentialType {
	return crypto.CredentialTypeX509
}

// CredentialProvider is implemented by sources of short-lived credential
// material. The provider name is used for cache keys and for looking up the
// provider implementation.
type CredentialProvider interface {
	// GetName returns the name of the credential provider.
	GetName() string
}

// JWTProvider is implemented by credential providers that can issue JWT
// credentials.
type JWTProvider interface {
	CredentialProvider

	// NewJWT returns a JWT credential for the identity configured through
	// the options.
	NewJWT(ctx context.Context, opts ...Option) (*JWT, error)
}

// X509Provider is implemented by credential providers that can issue X.509
// credentials.
type X509Provider interface {
	CredentialProvider

	// NewX509 returns an X.509 credential for the identity configured
	// through the options.
	NewX509(ctx context.Context, opts ...Option) (*X509, error)
}
