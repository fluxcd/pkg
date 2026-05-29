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

package signature_test

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"

	. "github.com/onsi/gomega"
	gossh "golang.org/x/crypto/ssh"

	"github.com/fluxcd/pkg/git/signature"
)

// sshKeyFactory returns a private key and the OpenSSH-PEM-encoded form of
// it. It accepts a passphrase argument: when non-nil the PEM is encrypted
// with it, otherwise it is unencrypted.
type sshKeyFactory func(t *testing.T, passphrase []byte) (crypto.PrivateKey, []byte)

func ed25519Key(t *testing.T, passphrase []byte) (crypto.PrivateKey, []byte) {
	t.Helper()
	g := NewWithT(t)
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	g.Expect(err).ToNot(HaveOccurred())
	return priv, marshalPEM(t, priv, passphrase)
}

func marshalPEM(t *testing.T, priv crypto.PrivateKey, passphrase []byte) []byte {
	t.Helper()
	g := NewWithT(t)
	var (
		block *pem.Block
		err   error
	)
	if len(passphrase) == 0 {
		block, err = gossh.MarshalPrivateKey(priv, "test key")
	} else {
		block, err = gossh.MarshalPrivateKeyWithPassphrase(priv, "test key", passphrase)
	}
	g.Expect(err).ToNot(HaveOccurred())
	return pem.EncodeToMemory(block)
}

func TestNewSSHSigner(t *testing.T) {
	tests := []struct {
		name           string
		key            sshKeyFactory
		pemPassphrase  []byte // passphrase used to encrypt the PEM; nil = unencrypted
		callPassphrase []byte // passphrase passed to NewSSHSigner; nil = none
		expectErr      string // substring expected in the error; empty = expect success
	}{
		{
			name: "ed25519 unencrypted",
			key:  ed25519Key,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			priv, pemBytes := tt.key(t, tt.pemPassphrase)

			signer, err := signature.NewSSHSigner(pemBytes, tt.callPassphrase)
			if tt.expectErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.expectErr))
				g.Expect(signer).To(BeNil())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(signer).ToNot(BeNil())

			// Round-trip: signer.Sign produces SSHSIG armor that verifies
			// against the matching authorized_keys-formatted public key.
			payload := []byte("hello sshsig world")
			sig, err := signer.Sign(bytes.NewReader(payload))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(string(sig)).To(HavePrefix("-----BEGIN SSH SIGNATURE-----"))

			gosshSigner, err := gossh.NewSignerFromKey(priv)
			g.Expect(err).ToNot(HaveOccurred())
			authorizedKey := gossh.MarshalAuthorizedKey(gosshSigner.PublicKey())

			_, err = signature.VerifySSHSignature(string(sig), payload, string(authorizedKey))
			g.Expect(err).ToNot(HaveOccurred())
		})
	}
}
