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

package secrets

import corev1 "k8s.io/api/core/v1"

const (
	// TLSCertKey is the standard key for TLS certificate data in secrets.
	TLSCertKey = corev1.TLSCertKey
	// TLSKeyKey is the standard key for TLS private key data in secrets.
	TLSKeyKey = corev1.TLSPrivateKeyKey
	// CACertKey is the standard key for CA certificate data in secrets.
	CACertKey = "ca.crt"

	// TLSCertFileKey is the deprecated key for TLS certificate data in secrets.
	TLSCertFileKey = "certFile"
	// TLSKeyFileKey is the deprecated key for TLS private key data in secrets.
	TLSKeyFileKey = "keyFile"
	// CACertFileKey is the deprecated key for CA certificate data in secrets.
	CACertFileKey = "caFile"

	// UsernameKey is the key for username data in basic auth secrets.
	UsernameKey = "username"
	// PasswordKey is the key for password data in basic auth secrets.
	PasswordKey = "password"

	// ProxyAddressKey is the key for proxy address data in proxy secrets.
	ProxyAddressKey = "address"

	// BearerTokenKey is the key for bearer token data in secrets.
	BearerTokenKey = "bearerToken"
	// TokenKey is the key for generic API token data in secrets.
	TokenKey = "token"
)
