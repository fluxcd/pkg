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

package signature

import (
	"slices"
	"strings"
)

// signatureType is the canonical string returned by GetSignatureType for
// each recognised signature category. The values are unexported on
// purpose: callers should compare against the strings returned by
// GetSignatureType, not against typed constants.
type signatureType string

const (
	// signatureTypePGP is returned for openPGP-armored signatures.
	signatureTypePGP signatureType = "openpgp"
	// signatureTypeSSH is returned for SSH-armored signatures.
	signatureTypeSSH signatureType = "ssh"
	// signatureTypeX509 is returned for X509/S-MIME-armored signatures.
	signatureTypeX509 signatureType = "x509"
	// signatureTypeUnknown is returned for armor that matches none of the
	// recognised prefixes.
	signatureTypeUnknown signatureType = "unknown"
	// signatureTypeEmpty is returned for the zero-length string.
	signatureTypeEmpty signatureType = "empty"
)

// X509SignaturePrefix is the prefix used by Git to identify x509 (S-MIME)
// signatures.
// https://github.com/git/git/blob/7b2bccb0d58d4f24705bf985de1f4612e4cf06e5/gpg-interface.c#L65
var X509SignaturePrefix = []string{"-----BEGIN SIGNED MESSAGE-----"}

// IsPGPSignature tests if the given signature is of type PGP.
// It returns true if the signature starts with the PGP signature prefix.
func IsPGPSignature(signature string) bool {
	return slices.ContainsFunc(PGPSignaturePrefix, func(prefix string) bool {
		return strings.HasPrefix(strings.TrimSpace(signature), prefix)
	})
}

// IsSSHSignature tests if the given signature is of type SSH.
// It returns true if the signature starts with the SSH signature prefix.
func IsSSHSignature(signature string) bool {
	return slices.ContainsFunc(SSHSignaturePrefix, func(prefix string) bool {
		return strings.HasPrefix(strings.TrimSpace(signature), prefix)
	})
}

// IsX509Signature tests if the given signature is of type x509 (S/MIME).
// It returns true if the signature starts with the x509 signature prefix.
//
// The signature package does not yet verify x509 signatures; this helper
// exists so [GetSignatureType] and the verify functions can report
// "x509" in their error messages, helping callers distinguish an x509
// signature from a corrupt or truly unknown one. Tracked upstream at
// https://github.com/fluxcd/source-controller/issues/1996.
func IsX509Signature(signature string) bool {
	return slices.ContainsFunc(X509SignaturePrefix, func(prefix string) bool {
		return strings.HasPrefix(strings.TrimSpace(signature), prefix)
	})
}

// IsEmptySignature tests if the given signature string is empty.
// It returns true if the signature string has a length of 0.
func IsEmptySignature(signature string) bool {
	return len(signature) == 0
}

// GetSignatureType returns the type of the signature as a string.
// It returns "openpgp" for PGP signatures, "ssh" for SSH signatures,
// "x509" for S/MIME signatures, "empty" for an empty signature
// and "unknown" for unrecognized signatures.
func GetSignatureType(signature string) string {
	if IsPGPSignature(signature) {
		return string(signatureTypePGP)
	}
	if IsSSHSignature(signature) {
		return string(signatureTypeSSH)
	}
	if IsX509Signature(signature) {
		return string(signatureTypeX509)
	}
	if IsEmptySignature(signature) {
		return string(signatureTypeEmpty)
	}
	return string(signatureTypeUnknown)
}
