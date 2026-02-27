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

package signatures

import (
	"strings"
)

// SignatureType represents the type of a signature.
type SignatureType string

const (
	// SignatureTypePGP represents a openPGP signature.
	SignatureTypePGP SignatureType = "openpgp"
	// SignatureTypeSSH represents an SSH signature.
	SignatureTypeSSH SignatureType = "ssh"
	// SignatureTypeX509 represents an x509 signature.
	SignatureTypeX509 SignatureType = "x509"
	// SignatureTypeUnknown represents an unknown signature type.
	SignatureTypeUnknown SignatureType = "unknown"
)

// Isx509Signature is the prefix used by Git to identify x509 signatures.
// https://github.com/git/git/blob/7b2bccb0d58d4f24705bf985de1f4612e4cf06e5/gpg-interface.c#L65
var X509SignaturePrefix = []string{"-----BEGIN SIGNED MESSAGE-----"}

func startsWithStrings(signature string, prefixList []string) bool {
	if signature == "" {
		return false
	}

	for _, prefix := range prefixList {
		if strings.HasPrefix(strings.TrimSpace(signature), prefix) {
			return true
		}
	}

	return false
}

// IsPGPSignature tests if the given signature is of type PGP.
// It returns true if the signature starts with the PGP signature prefix.
func IsPGPSignature(signature string) bool {
	return startsWithStrings(signature, PGPSignaturePrefix)
}

// IsSSHSignature tests if the given signature is of type SSH.
// It returns true if the signature starts with the SSH signature prefix.
func IsSSHSignature(signature string) bool {
	return startsWithStrings(signature, SSHSignaturePrefix)
}

// Isx509Signature tests if the given signature is of type x509.
// It returns true if the signature starts with the x509 signature prefix.
func Isx509Signature(signature string) bool {
	return startsWithStrings(signature, X509SignaturePrefix)
}

// GetSignatureType returns the type of the signature as a string.
// It returns "pgp" for PGP signatures, "ssh" for SSH signatures,
// and "unknown" for unrecognized or empty signatures.
func GetSignatureType(signature string) string {
	if IsPGPSignature(signature) {
		return string(SignatureTypePGP)
	}
	if IsSSHSignature(signature) {
		return string(SignatureTypeSSH)
	}
	if Isx509Signature(signature) {
		return string(SignatureTypeX509)
	}
	return string(SignatureTypeUnknown)
}
