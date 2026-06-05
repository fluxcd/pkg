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
	"crypto/rsa"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/hiddeco/sshsig"
	gossh "golang.org/x/crypto/ssh"
)

const SSHSignatureNamespace = "git"

// sshSignaturePrefix is the prefix used by Git to identify SSH signatures.
// https://github.com/git/git/blob/7b2bccb0d58d4f24705bf985de1f4612e4cf06e5/gpg-interface.c#L71
var sshSignaturePrefix = []string{"-----BEGIN SSH SIGNATURE-----"}

// ParseAuthorizedKeys parses the given authorized_keys-formatted string
// and returns the public keys it contains. Empty lines and lines whose
// first non-whitespace character is '#' are skipped.
//
// Parsing is fail-fast: if any non-comment line cannot be parsed as an
// SSH public key the function returns (nil, err), discarding any keys
// successfully parsed earlier in the input. This is intentional — a
// malformed entry typically indicates user error and silently dropping
// it would hide that. Callers that want best-effort behaviour should
// split the input themselves and call ParseAuthorizedKeys per line.
func ParseAuthorizedKeys(authorizedKeys string) ([]gossh.PublicKey, error) {
	var publicKeys []gossh.PublicKey

	for line := range strings.Lines(authorizedKeys) {
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

// VerifySSHSignature verifies the SSH signature against the payload using
// the provided authorized keys. It returns the fingerprint of the key that
// successfully verified the signature, or an error.
func VerifySSHSignature(signature string, payload []byte, authorizedKeys ...string) (string, error) {
	// Normalise leading/trailing whitespace once so the format-detection
	// helpers (which TrimSpace internally) and sshsig.Unarmor operate on
	// identical input.
	signature = strings.TrimSpace(signature)

	if signature == "" {
		return "", fmt.Errorf("unable to verify payload: %w", ErrSignatureEmpty)
	}

	if len(payload) == 0 {
		return "", fmt.Errorf("unable to verify payload: %w", ErrPayloadEmpty)
	}

	if !IsSSHSignature(signature) {
		return "", fmt.Errorf("unable to verify SSH signature, detected signature format: %s: %w", GetSignatureType(signature), ErrSignatureFormat)
	}

	// Unarmor the signature (remove PEM-like armor)
	sig, err := sshsig.Unarmor([]byte(signature))
	if err != nil {
		return "", fmt.Errorf("unable to unarmor SSH signature: %w", err)
	}

	// Track the first authorized_keys parse error. Only surfaced if no
	// authorized_keys input could be parsed at all; otherwise the no-match
	// error below takes precedence so that a malformed early entry does
	// not mask a later parseable one whose keys did not match.
	var (
		readAuthorizedKeysError error
		verifyAttempted         bool
		verifyErrors            []error
	)

	// Try to verify with each set of authorized keys
	for _, keys := range authorizedKeys {
		publicKeys, err := ParseAuthorizedKeys(keys)
		if err != nil {
			if readAuthorizedKeysError == nil {
				readAuthorizedKeysError = fmt.Errorf("unable to parse authorized keys: %w", err)
			}
			continue
		}
		verifyAttempted = true

		// Try to verify with each public key
		for _, pubKey := range publicKeys {
			// Verify the signature using sshsig library
			err := sshsig.Verify(bytes.NewReader(payload), sig, pubKey, sig.HashAlgorithm, SSHSignatureNamespace)
			if err == nil {
				// Signature verified successfully
				return gossh.FingerprintSHA256(pubKey), nil
			}
			verifyErrors = appendUniqueSentinel(verifyErrors, err)
		}
	}

	if !verifyAttempted && readAuthorizedKeysError != nil {
		return "", readAuthorizedKeysError
	}

	// Preserve the underlying sshsig sentinel errors (e.g.
	// sshsig.ErrPublicKeyMismatch, ErrNamespaceMismatch,
	// ErrUnsupportedHashAlgorithm) in the chain so callers can branch on
	// them via errors.Is.
	return "", fmt.Errorf("unable to verify payload with any of the given authorized keys: %w",
		errors.Join(append([]error{ErrNoMatchingKey}, verifyErrors...)...))
}

// SSHSigner adapts a [gossh.Signer] to the [Signer] interface, producing
// SSHSIG-armored signatures with namespace [SSHSignatureNamespace] ("git")
// and SHA-512 as the hash algorithm, matching Git's defaults for SSH-signed
// commits. Callers may type-assert a [Signer] returned by [NewSSHSigner]
// back to *SSHSigner to inspect or distinguish it from other Signer
// implementations.
type SSHSigner struct {
	inner gossh.Signer
}

// Sign produces an SSHSIG-armored signature over the message read from r,
// using SHA-512 and the "git" namespace.
func (s *SSHSigner) Sign(r io.Reader) ([]byte, error) {
	sig, err := sshsig.Sign(r, s.inner, sshsig.HashSHA512, SSHSignatureNamespace)
	if err != nil {
		return nil, err
	}
	return sshsig.Armor(sig), nil
}

// NewSSHSigner returns a [Signer] that signs commits with the given SSH
// private key. The pem argument is the private key in any format accepted
// by [gossh.ParsePrivateKey], typically the OpenSSH "-----BEGIN OPENSSH
// PRIVATE KEY-----" format produced by ssh-keygen. The passphrase argument
// is consulted only when the private key is encrypted; pass nil for an
// unencrypted key.
//
// Supported algorithms: ssh-ed25519, ecdsa-sha2-nistp256/384/521, and
// ssh-rsa with key size at least 2048 bits. DSA and undersized RSA keys
// are rejected at construction time because they produce signatures that
// modern OpenSSH refuses to verify.
//
// Signatures use namespace [SSHSignatureNamespace] ("git") and SHA-512,
// which match Git's defaults for SSH-signed commits. See
// https://git-scm.com/docs/gitformat-signature.
func NewSSHSigner(pem, passphrase []byte) (Signer, error) {
	inner, err := gossh.ParsePrivateKey(pem)
	if err != nil {
		var missingErr *gossh.PassphraseMissingError
		if !errors.As(err, &missingErr) {
			return nil, fmt.Errorf("could not parse SSH signing key: %w", err)
		}
		if len(passphrase) == 0 {
			return nil, ErrSSHPassphraseRequired
		}
		inner, err = gossh.ParsePrivateKeyWithPassphrase(pem, passphrase)
		if err != nil {
			return nil, fmt.Errorf("could not parse SSH signing key: %w", err)
		}
	}
	if err := validateSSHSigningKey(inner.PublicKey()); err != nil {
		return nil, err
	}
	return &SSHSigner{inner: inner}, nil
}

// validateSSHSigningKey rejects SSH public keys whose algorithm is not in
// the allowlist for commit signing.
//
// Allowlist:
//   - ssh-ed25519
//   - ecdsa-sha2-nistp256, ecdsa-sha2-nistp384, ecdsa-sha2-nistp521
//   - ssh-rsa (require >= 2048-bit key)
//
// DSA and undersized RSA are rejected because they produce signatures that
// modern OpenSSH (>= 8.7) refuses to verify.
func validateSSHSigningKey(pub gossh.PublicKey) error {
	switch pub.Type() {
	case gossh.KeyAlgoED25519,
		gossh.KeyAlgoECDSA256,
		gossh.KeyAlgoECDSA384,
		gossh.KeyAlgoECDSA521:
		return nil
	case gossh.KeyAlgoRSA:
		ck, ok := pub.(gossh.CryptoPublicKey)
		if !ok {
			return fmt.Errorf("unable to inspect RSA public key: type %T does not expose crypto.PublicKey", pub)
		}
		rsaPub, ok := ck.CryptoPublicKey().(*rsa.PublicKey)
		if !ok {
			return errors.New("unable to inspect RSA public key: not an *rsa.PublicKey")
		}
		if bits := rsaPub.Size() * 8; bits < 2048 {
			return fmt.Errorf("RSA key size %d bits is below the minimum supported by NewSSHSigner; must be at least 2048", bits)
		}
		return nil
	default:
		return fmt.Errorf("unsupported SSH signing key algorithm: %s", pub.Type())
	}
}

// appendUniqueSentinel appends err to dst only when no existing element of
// dst matches err under errors.Is. This keeps the joined error chain from
// growing linearly with the number of keys when every key fails with the
// same sshsig sentinel.
func appendUniqueSentinel(dst []error, err error) []error {
	for _, existing := range dst {
		if errors.Is(existing, err) {
			return dst
		}
	}
	return append(dst, err)
}
