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

import (
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	// ErrKeyNotFound is returned when a required key is not found in a secret.
	ErrKeyNotFound = errors.New("key not found in secret")
)

// KeyNotFoundError is returned when a specific key is not found in a secret.
type KeyNotFoundError struct {
	Key    string
	Secret *corev1.Secret
}

func (e *KeyNotFoundError) Error() string {
	return fmt.Sprintf("secret '%s': key '%s' not found", client.ObjectKeyFromObject(e.Secret), e.Key)
}

func (e *KeyNotFoundError) Is(target error) bool {
	return errors.Is(target, ErrKeyNotFound)
}

// TLSValidationError represents TLS certificate validation errors.
type TLSValidationError struct {
	Type TLSValidationErrorType
}

// TLSValidationErrorType defines the type of TLS validation error.
type TLSValidationErrorType int

const (
	// ErrMissingPrivateKey indicates that a certificate exists but the private key is missing.
	ErrMissingPrivateKey TLSValidationErrorType = iota
	// ErrMissingCertificate indicates that a private key exists but the certificate is missing.
	ErrMissingCertificate
	// ErrNoCertificatePairOrCA indicates that neither a certificate pair nor a CA certificate is present.
	ErrNoCertificatePairOrCA
)

func (e *TLSValidationError) Error() string {
	switch e.Type {
	case ErrMissingPrivateKey:
		return "found certificate but missing private key"
	case ErrMissingCertificate:
		return "found private key but missing certificate"
	case ErrNoCertificatePairOrCA:
		return "no CA certificate or client certificate pair found"
	default:
		return "TLS validation error"
	}
}

// SecretTLSValidationError wraps TLSValidationError with secret reference information.
type SecretTLSValidationError struct {
	*TLSValidationError
	Secret *corev1.Secret
}

func (e *SecretTLSValidationError) Error() string {
	ref := client.ObjectKeyFromObject(e.Secret)

	switch e.Type {
	case ErrMissingPrivateKey:
		return fmt.Sprintf("secret '%s' contains '%s' but missing '%s'", ref, KeyTLSCert, KeyTLSPrivateKey)
	case ErrMissingCertificate:
		return fmt.Sprintf("secret '%s' contains '%s' but missing '%s'", ref, KeyTLSPrivateKey, KeyTLSCert)
	case ErrNoCertificatePairOrCA:
		return fmt.Sprintf("secret '%s' must contain either '%s' or both '%s' and '%s'", ref, KeyCACert, KeyTLSCert, KeyTLSPrivateKey)
	default:
		return "TLS validation error"
	}
}

func (e *SecretTLSValidationError) Unwrap() error {
	return e.TLSValidationError
}

// BasicAuthMalformedError is returned when a secret contains partial basic auth data.
// This indicates a configuration error where one of username/password is present but the other is missing.
type BasicAuthMalformedError struct {
	Present string
	Missing string
	Secret  *corev1.Secret
}

func (e *BasicAuthMalformedError) Error() string {
	ref := client.ObjectKeyFromObject(e.Secret)
	return fmt.Sprintf("secret '%s': malformed basic auth - has '%s' but missing '%s'",
		ref, e.Present, e.Missing)
}
