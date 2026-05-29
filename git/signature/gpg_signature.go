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
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// pgpSignaturePrefix is the prefix used by Git to identify PGP signatures.
// https://github.com/git/git/blob/7b2bccb0d58d4f24705bf985de1f4612e4cf06e5/gpg-interface.c#L56
//
// PGP MESSAGE armor is intentionally not included: Git's gpgsig field only
// carries detached signatures, which use the PGP SIGNATURE armor type, and
// the underlying openpgp.CheckArmoredDetachedSignature rejects MESSAGE armor.
// Detecting it here would only produce a misleading "no matching key" error
// downstream.
var pgpSignaturePrefix = []string{
	"-----BEGIN PGP SIGNATURE-----",
}

// VerifyPGPSignature verifies the PGP signature against the payload using
// the provided key rings. It returns the key ID of the key that
// successfully verified the signature, or an error.
func VerifyPGPSignature(signature string, payload []byte, keyRings ...string) (string, error) {
	// Normalise leading/trailing whitespace once so the format-detection
	// helpers (which TrimSpace internally) and the underlying openpgp
	// armor decoder operate on identical input.
	signature = strings.TrimSpace(signature)

	if signature == "" {
		return "", fmt.Errorf("unable to verify payload: %w", ErrSignatureEmpty)
	}

	if len(payload) == 0 {
		return "", fmt.Errorf("unable to verify payload: %w", ErrPayloadEmpty)
	}

	if !IsPGPSignature(signature) {
		return "", fmt.Errorf("unable to verify openPGP signature, detected signature format: %s: %w", GetSignatureType(signature), ErrSignatureFormat)
	}

	// Track the first key ring parse error. Only surfaced if no key ring
	// could be parsed at all; otherwise the no-match error below takes
	// precedence so that a malformed early entry does not mask a later
	// parseable one whose keys did not match.
	var (
		readKeyRingError error
		verifyAttempted  bool
	)

	for _, r := range keyRings {
		reader := strings.NewReader(r)
		keyring, err := openpgp.ReadArmoredKeyRing(reader)
		if err != nil {
			if readKeyRingError == nil {
				readKeyRingError = fmt.Errorf("unable to read armored key ring: %w", err)
			}
			continue
		}
		verifyAttempted = true
		signer, err := openpgp.CheckArmoredDetachedSignature(keyring, bytes.NewReader(payload), strings.NewReader(signature), nil)
		if err == nil {
			return signer.PrimaryKey.KeyIdString(), nil
		}
	}

	if !verifyAttempted && readKeyRingError != nil {
		return "", readKeyRingError
	}

	return "", fmt.Errorf("unable to verify payload with any of the given key rings: %w", ErrNoMatchingKey)
}

// OpenPGPSigner adapts an [openpgp.Entity] to the [Signer] interface so it
// can be used as a generic Git commit signer. Callers may type-assert a
// [Signer] returned by [NewOpenPGPSigner] back to *OpenPGPSigner to inspect
// or distinguish it from other Signer implementations.
type OpenPGPSigner struct {
	entity *openpgp.Entity
}

// Sign produces an ASCII-armored detached OpenPGP signature over the
// message read from r. The output matches what go-git's internal gpgSigner
// produces, so it is interchangeable with the previous typed-entity
// signing path.
func (s *OpenPGPSigner) Sign(r io.Reader) ([]byte, error) {
	var buf bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&buf, s.entity, r, nil); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NewOpenPGPSigner returns a [Signer] that signs commits with the given
// OpenPGP entity. The entity's private key must be present and decrypted;
// callers that load keys from passphrase-protected key rings are
// responsible for decryption before constructing the signer.
//
// The constructor returns an error only when the entity is nil; today no
// other validation is performed, but the (Signer, error) shape leaves room
// to add validation later without an API break.
func NewOpenPGPSigner(e *openpgp.Entity) (Signer, error) {
	if e == nil {
		return nil, errors.New("nil openpgp entity")
	}
	return &OpenPGPSigner{entity: e}, nil
}
