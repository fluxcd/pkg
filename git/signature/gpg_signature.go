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
	"fmt"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// PGPSignaturePrefix is the prefix used by Git to identify PGP signatures.
// https://github.com/git/git/blob/7b2bccb0d58d4f24705bf985de1f4612e4cf06e5/gpg-interface.c#L56
var PGPSignaturePrefix = []string{
	"-----BEGIN PGP SIGNATURE-----",
	"-----BEGIN PGP MESSAGE-----",
}

// VerifyPGPSignature verifies the PGP signature against the payload using
// the provided key rings. It returns the key ID of the key that
// successfully verified the signature, or an error.
func VerifyPGPSignature(signature string, payload []byte, keyRings ...string) (string, error) {
	if signature == "" {
		return "", fmt.Errorf("unable to verify payload as the provided signature is empty")
	}

	if len(payload) == 0 {
		return "", fmt.Errorf("unable to verify payload as the provided payload is empty")
	}

	if !IsPGPSignature(signature) {
		return "", fmt.Errorf("unable to verify openPGP signature, detected signature format: %s", GetSignatureType(signature))
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

	return "", fmt.Errorf("unable to verify payload with any of the given key rings")
}
