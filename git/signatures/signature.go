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
	// SignatureTypePGP represents a PGP signature.
	SignatureTypePGP SignatureType = "pgp"
	// SignatureTypeSSH represents an SSH signature.
	SignatureTypeSSH SignatureType = "ssh"
	// SignatureTypeUnknown represents an unknown signature type.
	SignatureTypeUnknown SignatureType = "unknown"
)

// IsPGPSignature tests if the given signature is of type PGP.
// It returns true if the signature starts with the PGP signature prefix.
func IsPGPSignature(signature string) bool {
	if signature == "" {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(signature), PGPSignaturePrefix)
}

// IsSSHSignature tests if the given signature is of type SSH.
// It returns true if the signature starts with the SSH signature prefix.
func IsSSHSignature(signature string) bool {
	if signature == "" {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(signature), SSHSignaturePrefix)
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
	return string(SignatureTypeUnknown)
}
