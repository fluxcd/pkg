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
	return fmt.Sprintf("secret '%s': key '%s' not found", secretRef(e.Secret), e.Key)
}

func (e *KeyNotFoundError) Is(target error) bool {
	return errors.Is(target, ErrKeyNotFound)
}

// secretRef returns a string representation of a secret reference.
func secretRef(secret *corev1.Secret) string {
	return fmt.Sprintf("%s/%s", secret.Namespace, secret.Name)
}

// serviceAccountRef returns a string representation of a service account reference.
func serviceAccountRef(sa *corev1.ServiceAccount) string {
	return fmt.Sprintf("%s/%s", sa.Namespace, sa.Name)
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

// enhanceSecretValidationError enhances TLS validation errors with secret reference information.
func enhanceSecretValidationError(err error, secret *corev1.Secret) error {
	var tlsErr *TLSValidationError
	if !errors.As(err, &tlsErr) {
		return err
	}

	ref := secretRef(secret)

	switch tlsErr.Type {
	case ErrMissingPrivateKey:
		return fmt.Errorf("secret '%s' contains '%s' but missing '%s'", ref, TLSCertKey, TLSKeyKey)
	case ErrMissingCertificate:
		return fmt.Errorf("secret '%s' contains '%s' but missing '%s'", ref, TLSKeyKey, TLSCertKey)
	case ErrNoCertificatePairOrCA:
		return fmt.Errorf("secret '%s' must contain either '%s' or both '%s' and '%s'", ref, CACertKey, TLSCertKey, TLSKeyKey)
	default:
		return err
	}
}
