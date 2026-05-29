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
	"strings"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/git/signature"
)

func TestNewOpenPGPSigner(t *testing.T) {
	t.Run("nil entity returns error", func(t *testing.T) {
		g := NewWithT(t)

		_, err := signature.NewOpenPGPSigner(nil)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("nil openpgp entity"))
	})

	t.Run("sign produces detached armored signature verifiable by VerifyPGPSignature", func(t *testing.T) {
		g := NewWithT(t)

		entity, err := openpgp.NewEntity("Test", "test signing key", "test@example.com", nil)
		g.Expect(err).ToNot(HaveOccurred())

		signer, err := signature.NewOpenPGPSigner(entity)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(signer).ToNot(BeNil())

		payload := []byte("hello world")
		sig, err := signer.Sign(bytes.NewReader(payload))
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(sig).ToNot(BeEmpty())
		g.Expect(string(sig)).To(HavePrefix("-----BEGIN PGP SIGNATURE-----"))

		// Round-trip: armor the public key and verify the produced signature.
		var sb strings.Builder
		armorWriter, err := armor.Encode(&sb, "PGP PUBLIC KEY BLOCK", nil)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(entity.Serialize(armorWriter)).To(Succeed())
		g.Expect(armorWriter.Close()).To(Succeed())

		_, err = signature.VerifyPGPSignature(string(sig), payload, sb.String())
		g.Expect(err).ToNot(HaveOccurred())
	})
}
