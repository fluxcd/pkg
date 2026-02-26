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
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/hiddeco/sshsig"
	gossh "golang.org/x/crypto/ssh"
)

// SSHSignaturePrefix is the prefix used by Git to identify SSH signatures.
const SSHSignaturePrefix = "-----BEGIN SSH SIGNATURE-----"

// ParseAuthorizedKeys parses the given authorized keys string and returns
// a slice of public keys. It supports comments and empty lines.
func ParseAuthorizedKeys(authorizedKeys string) ([]gossh.PublicKey, error) {
	var publicKeys []gossh.PublicKey

	for _, line := range strings.Split(authorizedKeys, "\n") {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse the authorized key line
		pubKey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(line))
		if err != nil {
			return nil, fmt.Errorf("unable to parse authorized key: %w", err)
		}

		publicKeys = append(publicKeys, pubKey)
	}

	return publicKeys, nil
}

// verifySSHSignature verifies the SSH signature against the payload using
// the provided authorized keys. It returns the fingerprint of the key that
// successfully verified the signature, or an error.
func VerifySSHSignature(signature string, payload []byte, authorizedKeys ...string) (string, error) {
	if signature == "" {
		return "", fmt.Errorf("unable to verify payload as the provided signature is empty")
	}

	if len(payload) == 0 {
		return "", fmt.Errorf("unable to verify payload as the provided payload is empty")
	}

	// Unarmor the signature (remove PEM-like armor)
	sig, err := sshsig.Unarmor([]byte(signature))
	if err != nil {
		return "", fmt.Errorf("unable to unarmor SSH signature: %w", err)
	}

	// Try to verify with each set of authorized keys
	for _, keys := range authorizedKeys {
		publicKeys, err := ParseAuthorizedKeys(keys)
		if err != nil {
			return "", fmt.Errorf("unable to parse authorized keys: %w", err)
		}

		// Try to verify with each public key
		for _, pubKey := range publicKeys {
			// Verify the signature using sshsig library
			// The namespace for Git is "git"
			// Git supports both SHA256 and SHA512, so we try both
			for _, hashAlgo := range []sshsig.HashAlgorithm{sshsig.HashSHA256, sshsig.HashSHA512} {
				err := sshsig.Verify(bytes.NewReader(payload), sig, pubKey, hashAlgo, "git")
				if err == nil {
					// Signature verified successfully
					return GetPublicKeyFingerprint(pubKey), nil
				}
			}
		}
	}

	return "", fmt.Errorf("unable to verify payload with any of the given authorized keys")
}

// getPublicKeyFingerprint returns the SHA256 fingerprint of the public key
// in the format used by SSH (e.g., "SHA256:abc123...").
func GetPublicKeyFingerprint(pubKey gossh.PublicKey) string {
	hash := sha256.Sum256(pubKey.Marshal())
	return "SHA256:" + base64.StdEncoding.EncodeToString(hash[:])
}
