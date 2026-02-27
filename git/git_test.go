/*
Copyright 2021 The Flux authors

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

package git

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fluxcd/pkg/git/testutils"
	"github.com/go-git/go-git/v5/plumbing"
	. "github.com/onsi/gomega"
)

const (
	signaturePGPSignature                      = "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----"
	signaturePGPMessage                        = "-----BEGIN PGP MESSAGE-----\n-----END PGP MESSAGE-----"
	signatureSSH                               = "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----"
	signatureX509                              = "-----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----"
	signatureUnknown                           = "-----BEGIN UNKNOWN SIGNATURE-----\n-----END UNKNOWN SIGNATURE-----"
	signaturePGPSignatureWithLeadingWhitespace = "  " + signaturePGPSignature
	signatureSSHWithLeadingWhitespace          = "  " + signatureSSH
)

func TestHash_Algorithm(t *testing.T) {
	tests := []struct {
		name string
		hash Hash
		want string
	}{
		{
			name: "SHA-1",
			hash: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
			want: HashTypeSHA1,
		},
		{
			name: "SHA-256",
			hash: Hash("6ee9a7ade2ca791bc1bf9d133ef6ddaa9097cf521e6a19be92dbcc3f2e82f6d8"),
			want: HashTypeUnknown,
		},
		{
			name: "MD5",
			hash: Hash("dba535cd50b291777a055338572e4a4b"),
			want: HashTypeUnknown,
		},
		{
			name: "Empty",
			hash: Hash(""),
			want: HashTypeUnknown,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(tt.hash.Algorithm()).To(Equal(tt.want))
		})
	}
}

func TestHash_Digest(t *testing.T) {
	tests := []struct {
		name string
		hash Hash
		want string
	}{
		{
			name: "With a SHA-1 hash",
			hash: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
			want: "sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "With an unknown (MD5) hash",
			hash: Hash("dba535cd50b291777a055338572e4a4b"),
			want: "<unknown>:dba535cd50b291777a055338572e4a4b",
		},
		{
			name: "With an unknown (SHA-256) hash",
			hash: Hash("6ee9a7ade2ca791bc1bf9d133ef6ddaa9097cf521e6a19be92dbcc3f2e82f6d8"),
			want: "<unknown>:6ee9a7ade2ca791bc1bf9d133ef6ddaa9097cf521e6a19be92dbcc3f2e82f6d8",
		},
		{
			name: "With a nil hash",
			hash: nil,
			want: "",
		},
		{
			name: "With an empty hash",
			hash: Hash(""),
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(tt.hash.Digest()).To(Equal(tt.want))
		})
	}
}

func TestCommit_String(t *testing.T) {
	tests := []struct {
		name   string
		commit *Commit
		want   string
	}{
		{
			name: "Reference and commit",
			commit: &Commit{
				Hash:      []byte("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
				Reference: "refs/heads/main",
			},
			want: "main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "Reference with slash and commit",
			commit: &Commit{
				Hash:      []byte("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
				Reference: "refs/heads/feature/branch",
			},
			want: "feature/branch@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "No name reference",
			commit: &Commit{
				Hash: []byte("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
			},
			want: "sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(tt.commit.String()).To(Equal(tt.want))
		})
	}
}

func TestCommit_AbsoluteReference(t *testing.T) {
	tests := []struct {
		name   string
		commit *Commit
		want   string
	}{
		{
			name: "Reference and commit",
			commit: &Commit{
				Hash:      []byte("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
				Reference: "refs/heads/main",
			},
			want: "refs/heads/main@sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "No name reference",
			commit: &Commit{
				Hash: []byte("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
			},
			want: "sha1:5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			g.Expect(tt.commit.AbsoluteReference()).To(Equal(tt.want))
		})
	}
}

func TestCommit_ShortMessage(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "short message",
			input: "a short commit message",
			want:  "a short commit message",
		},
		{
			name:  "long message",
			input: "hello world - a long commit message for testing long messages",
			want:  "hello world - a long commit message for testing lo...",
		},
		{
			name: "multi line commit message",
			input: `title of the commit

detailed description
of the commit`,
			want: "title of the commit",
		},
		{
			name:  "message with unicodes",
			input: "a message with unicode characters 你好世界 🏞️ 🏕️ ⛩️ 🌌",
			want:  "a message with unicode characters 你好世界 🏞️ 🏕️ ⛩️ 🌌",
		},
		{
			name:  "empty commit message",
			input: "",
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			c := Commit{Message: tt.input}
			g.Expect(c.ShortMessage()).To(Equal(tt.want))
		})
	}
}

func TestIsConcreteCommit(t *testing.T) {
	tests := []struct {
		name   string
		commit Commit
		result bool
	}{
		{
			name: "concrete commit",
			commit: Commit{
				Hash:      Hash("foo"),
				Reference: "refs/tags/main",
				Author: Signature{
					Name: "user", Email: "user@example.com", When: time.Now(),
				},
				Committer: Signature{
					Name: "user", Email: "user@example.com", When: time.Now(),
				},
				Signature: "signature",
				Encoded:   []byte("commit-content"),
				Message:   "commit-message",
			},
			result: true,
		},
		{
			name:   "partial commit",
			commit: Commit{Hash: Hash("foo")},
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(IsConcreteCommit(tt.commit)).To(Equal(tt.result))
		})
	}
}

func TestIsAnnotatedTag(t *testing.T) {
	tests := []struct {
		name   string
		tag    Tag
		result bool
	}{
		{
			name: "annotated tag",
			tag: Tag{
				Hash:    Hash("foo"),
				Name:    "v1.0.0",
				Encoded: []byte("tag-content"),
			},
			result: true,
		},
		{
			name: "lightweight tag",
			tag: Tag{
				Hash: Hash("foo"),
				Name: "v1.0.0",
			},
			result: false,
		},
		{
			name: "empty encoded",
			tag: Tag{
				Hash:    Hash("foo"),
				Name:    "v1.0.0",
				Encoded: []byte{},
			},
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(IsAnnotatedTag(tt.tag)).To(Equal(tt.result))
		})
	}
}

func TestIsSignedTag(t *testing.T) {
	tests := []struct {
		name   string
		tag    Tag
		result bool
	}{
		{
			name: "signed tag",
			tag: Tag{
				Hash:      Hash("foo"),
				Name:      "v1.0.0",
				Signature: signaturePGPSignature,
			},
			result: true,
		},
		{
			name: "unsigned tag",
			tag: Tag{
				Hash: Hash("foo"),
				Name: "v1.0.0",
			},
			result: false,
		},
		{
			name: "empty signature",
			tag: Tag{
				Hash:      Hash("foo"),
				Name:      "v1.0.0",
				Signature: "",
			},
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(IsSignedTag(tt.tag)).To(Equal(tt.result))
		})
	}
}

func TestTag_String(t *testing.T) {
	tests := []struct {
		name string
		tag  *Tag
		want string
	}{
		{
			name: "annotated tag with hash",
			tag: &Tag{
				Hash: Hash("5394cb7f48332b2de7c17dd8b8384bbc84b7e738"),
				Name: "v1.0.0",
			},
			want: "v1.0.0@5394cb7f48332b2de7c17dd8b8384bbc84b7e738",
		},
		{
			name: "lightweight tag without hash",
			tag: &Tag{
				Name: "v1.0.0",
			},
			want: "v1.0.0",
		},
		{
			name: "tag with empty hash",
			tag: &Tag{
				Hash: Hash(""),
				Name: "v2.0.0",
			},
			want: "v2.0.0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.tag.String()).To(Equal(tt.want))
		})
	}
}

func TestIsSigned(t *testing.T) {
	tests := []struct {
		name          string
		commit        *Commit
		tag           *Tag
		wantPGPCommit bool
		wantSSHCommit bool
		wantPGPTag    bool
		wantSSHTag    bool
	}{
		{
			name: "PGP signed with SIGNATURE prefix",
			commit: &Commit{
				Signature: signaturePGPSignature,
			},
			tag: &Tag{
				Signature: signaturePGPSignature,
			},
			wantPGPCommit: true,
			wantSSHCommit: false,
			wantPGPTag:    true,
			wantSSHTag:    false,
		},
		{
			name: "PGP signed with MESSAGE prefix",
			commit: &Commit{
				Signature: signaturePGPMessage,
			},
			tag: &Tag{
				Signature: signaturePGPMessage,
			},
			wantPGPCommit: true,
			wantSSHCommit: false,
			wantPGPTag:    true,
			wantSSHTag:    false,
		},
		{
			name: "SSH signed",
			commit: &Commit{
				Signature: signatureSSH,
			},
			tag: &Tag{
				Signature: signatureSSH,
			},
			wantPGPCommit: false,
			wantSSHCommit: true,
			wantPGPTag:    false,
			wantSSHTag:    true,
		},
		{
			name: "X509 signed",
			commit: &Commit{
				Signature: signatureX509,
			},
			tag: &Tag{
				Signature: signatureX509,
			},
			wantPGPCommit: false,
			wantSSHCommit: false,
			wantPGPTag:    false,
			wantSSHTag:    false,
		},
		{
			name:          "unsigned",
			commit:        &Commit{},
			tag:           &Tag{},
			wantPGPCommit: false,
			wantSSHCommit: false,
			wantPGPTag:    false,
			wantSSHTag:    false,
		},
		{
			name: "PGP signed with leading whitespace",
			commit: &Commit{
				Signature: signaturePGPSignatureWithLeadingWhitespace,
			},
			tag: &Tag{
				Signature: signaturePGPSignatureWithLeadingWhitespace,
			},
			wantPGPCommit: true,
			wantSSHCommit: false,
			wantPGPTag:    true,
			wantSSHTag:    false,
		},
		{
			name: "SSH signed with leading whitespace",
			commit: &Commit{
				Signature: signatureSSHWithLeadingWhitespace,
			},
			tag: &Tag{
				Signature: signatureSSHWithLeadingWhitespace,
			},
			wantPGPCommit: false,
			wantSSHCommit: true,
			wantPGPTag:    false,
			wantSSHTag:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.commit.IsPGPSigned()).To(Equal(tt.wantPGPCommit))
			g.Expect(tt.commit.IsSSHSigned()).To(Equal(tt.wantSSHCommit))
			g.Expect(tt.tag.IsPGPSigned()).To(Equal(tt.wantPGPTag))
			g.Expect(tt.tag.IsSSHSigned()).To(Equal(tt.wantSSHTag))
		})
	}
}

func TestSignatureType(t *testing.T) {
	tests := []struct {
		name   string
		commit *Commit
		tag    *Tag
		want   string
	}{
		{
			name: "PGP signed with SIGNATURE prefix",
			commit: &Commit{
				Signature: signaturePGPSignature,
			},
			tag: &Tag{
				Signature: signaturePGPSignature,
			},
			want: "openpgp",
		},
		{
			name: "PGP signed with MESSAGE prefix",
			commit: &Commit{
				Signature: signaturePGPMessage,
			},
			tag: &Tag{
				Signature: signaturePGPMessage,
			},
			want: "openpgp",
		},
		{
			name: "SSH signed",
			commit: &Commit{
				Signature: signatureSSH,
			},
			tag: &Tag{
				Signature: signatureSSH,
			},
			want: "ssh",
		},
		{
			name: "X509 signed",
			commit: &Commit{
				Signature: signatureX509,
			},
			tag: &Tag{
				Signature: signatureX509,
			},
			want: "x509",
		},
		{
			name:   "unsigned",
			commit: &Commit{},
			tag:    &Tag{},
			want:   "unknown",
		},
		{
			name: "unknown signature type",
			commit: &Commit{
				Signature: signatureUnknown,
			},
			tag: &Tag{
				Signature: signatureUnknown,
			},
			want: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.commit.SignatureType()).To(Equal(tt.want))
			g.Expect(tt.tag.SignatureType()).To(Equal(tt.want))
		})
	}
}

func TestCommit_VerifyGPG(t *testing.T) {
	testDataDir := filepath.Join("signatures", "testdata", "gpg_signatures")

	tests := []struct {
		name    string
		sigFile string
		keyFile string
		wantErr string
	}{
		{
			name:    "valid PGP signature",
			sigFile: "commit_rsa_2048_signed.txt",
			keyFile: "key_rsa_2048.pub",
		},
		{
			name:    "missing signature",
			sigFile: "commit_unsigned.txt",
			keyFile: "key_rsa_2048.pub",
			wantErr: "unable to verify Git commit: unable to verify payload as the provided signature is empty",
		},
		{
			name:    "invalid signature",
			sigFile: "commit_rsa_2048_signed.txt",
			keyFile: "key_ed25519.pub",
			wantErr: "unable to verify Git commit: unable to verify payload with any of the given key rings",
		},
		{
			name:    "no key rings provided",
			sigFile: "commit_rsa_2048_signed.txt",
			wantErr: "unable to verify Git commit: unable to verify payload with any of the given key rings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Parse the commit from the fixture file
			commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, tt.sigFile))
			g.Expect(err).ToNot(HaveOccurred())

			// Create a git.Commit from the parsed object
			encoded := &plumbing.MemoryObject{}
			err = commitObj.EncodeWithoutSignature(encoded)
			g.Expect(err).ToNot(HaveOccurred())
			reader, err := encoded.Reader()
			g.Expect(err).ToNot(HaveOccurred())
			b, err := io.ReadAll(reader)
			g.Expect(err).ToNot(HaveOccurred())

			gitCommit := &Commit{
				Signature: commitObj.PGPSignature,
				Encoded:   b,
			}

			// Prepare key rings
			var keyRings []string
			if tt.keyFile != "" {
				publicKey, err := os.ReadFile(filepath.Join(testDataDir, tt.keyFile))
				g.Expect(err).ToNot(HaveOccurred())
				keyRings = append(keyRings, string(publicKey))
			}

			// get result from deprecated function
			depFingerprint, depErr := gitCommit.Verify(keyRings...)

			// Verify the signature using the git.Commit's VerifyGPG method
			fingerprint, err := gitCommit.VerifyGPG(keyRings...)

			g.Expect(fingerprint).To(ContainSubstring(depFingerprint))
			if err == nil {
				g.Expect(depErr).ToNot(HaveOccurred())
			} else {
				g.Expect(err.Error()).To(ContainSubstring(depErr.Error()))
			}

			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				g.Expect(fingerprint).To(BeEmpty())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(fingerprint).ToNot(BeEmpty())
		})
	}
}

func TestTag_VerifyGPG(t *testing.T) {
	testDataDir := filepath.Join("signatures", "testdata", "gpg_signatures")

	tests := []struct {
		name    string
		sigFile string
		keyFile string
		wantErr string
	}{
		{
			name:    "valid PGP signature",
			sigFile: "tag_rsa_2048_signed.txt",
			keyFile: "key_rsa_2048.pub",
		},
		{
			name:    "missing signature",
			sigFile: "commit_unsigned.txt",
			keyFile: "key_rsa_2048.pub",
			wantErr: "unable to verify Git tag: unable to verify payload as the provided signature is empty",
		},
		{
			name:    "invalid signature",
			sigFile: "tag_rsa_2048_signed.txt",
			keyFile: "key_ed25519.pub",
			wantErr: "unable to verify Git tag: unable to verify payload with any of the given key rings",
		},
		{
			name:    "no key rings provided",
			sigFile: "tag_rsa_2048_signed.txt",
			wantErr: "unable to verify Git tag: unable to verify payload with any of the given key rings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Parse the tag from the fixture file
			tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, tt.sigFile))
			g.Expect(err).ToNot(HaveOccurred())

			// Create a git.Tag from the parsed object
			encoded := &plumbing.MemoryObject{}
			err = tagObj.EncodeWithoutSignature(encoded)
			g.Expect(err).ToNot(HaveOccurred())
			reader, err := encoded.Reader()
			g.Expect(err).ToNot(HaveOccurred())
			b, err := io.ReadAll(reader)
			g.Expect(err).ToNot(HaveOccurred())

			gitTag := &Tag{
				Signature: tagObj.PGPSignature,
				Encoded:   b,
			}

			// Prepare key rings
			var keyRings []string
			if tt.keyFile != "" {
				publicKey, err := os.ReadFile(filepath.Join(testDataDir, tt.keyFile))
				g.Expect(err).ToNot(HaveOccurred())
				keyRings = append(keyRings, string(publicKey))
			}

			// get result from deprecated function
			depFingerprint, depErr := gitTag.Verify(keyRings...)

			// Verify the signature using the git.Tag's VerifyGPG method
			fingerprint, err := gitTag.VerifyGPG(keyRings...)

			g.Expect(fingerprint).To(ContainSubstring(depFingerprint))
			if err == nil {
				g.Expect(depErr).ToNot(HaveOccurred())
			} else {
				g.Expect(err.Error()).To(ContainSubstring(depErr.Error()))
			}

			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				g.Expect(fingerprint).To(BeEmpty())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(fingerprint).ToNot(BeEmpty())
		})
	}
}

func TestCommit_VerifySSH(t *testing.T) {
	testDataDir := filepath.Join("signatures", "testdata", "ssh_signatures")

	tests := []struct {
		name           string
		sigFile        string
		authorizedKeys string
		wantErr        string
	}{
		{
			name:           "valid SSH signature",
			sigFile:        "commit_rsa_signed.txt",
			authorizedKeys: "authorized_keys_rsa",
		},
		{
			name:           "missing signature",
			sigFile:        "commit_unsigned.txt",
			authorizedKeys: "authorized_keys_rsa",
			wantErr:        "unable to verify Git commit SSH signature: unable to verify payload as the provided signature is empty",
		},
		{
			name:           "invalid signature",
			sigFile:        "commit_rsa_signed.txt",
			authorizedKeys: "authorized_keys_ed25519",
			wantErr:        "unable to verify Git commit SSH signature: unable to verify payload with any of the given authorized keys",
		},
		{
			name:    "no authorized keys provided",
			sigFile: "commit_rsa_signed.txt",
			wantErr: "unable to verify Git commit SSH signature: unable to verify payload with any of the given authorized keys",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Parse the commit from the fixture file
			commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, tt.sigFile))
			g.Expect(err).ToNot(HaveOccurred())

			// Create a git.Commit from the parsed object
			encoded := &plumbing.MemoryObject{}
			err = commitObj.EncodeWithoutSignature(encoded)
			g.Expect(err).ToNot(HaveOccurred())
			reader, err := encoded.Reader()
			g.Expect(err).ToNot(HaveOccurred())
			b, err := io.ReadAll(reader)
			g.Expect(err).ToNot(HaveOccurred())

			gitCommit := &Commit{
				Signature: commitObj.PGPSignature,
				Encoded:   b,
			}

			// Prepare authorized keys
			var authorizedKeys []string
			if tt.authorizedKeys != "" {
				authorizedKey, err := os.ReadFile(filepath.Join(testDataDir, tt.authorizedKeys))
				g.Expect(err).ToNot(HaveOccurred())
				authorizedKeys = append(authorizedKeys, string(authorizedKey))
			}

			// Verify the signature using the git.Commit's VerifySSH method
			fingerprint, err := gitCommit.VerifySSH(authorizedKeys...)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				g.Expect(fingerprint).To(BeEmpty())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(fingerprint).ToNot(BeEmpty())
		})
	}
}

func TestTag_VerifySSH(t *testing.T) {
	testDataDir := filepath.Join("signatures", "testdata", "ssh_signatures")

	tests := []struct {
		name           string
		sigFile        string
		authorizedKeys string
		wantErr        string
	}{
		{
			name:           "valid SSH signature",
			sigFile:        "tag_rsa_signed.txt",
			authorizedKeys: "authorized_keys_rsa",
		},
		{
			name:           "missing signature",
			sigFile:        "commit_unsigned.txt",
			authorizedKeys: "authorized_keys_rsa",
			wantErr:        "unable to verify Git tag SSH signature: unable to verify payload as the provided signature is empty",
		},
		{
			name:           "invalid signature",
			sigFile:        "tag_rsa_signed.txt",
			authorizedKeys: "authorized_keys_ed25519",
			wantErr:        "unable to verify Git tag SSH signature: unable to verify payload with any of the given authorized keys",
		},
		{
			name:    "no authorized keys provided",
			sigFile: "tag_rsa_signed.txt",
			wantErr: "unable to verify Git tag SSH signature: unable to verify payload with any of the given authorized keys",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Parse the tag from the fixture file
			tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, tt.sigFile))
			g.Expect(err).ToNot(HaveOccurred())

			// Create a git.Tag from the parsed object
			encoded := &plumbing.MemoryObject{}
			err = tagObj.EncodeWithoutSignature(encoded)
			g.Expect(err).ToNot(HaveOccurred())
			reader, err := encoded.Reader()
			g.Expect(err).ToNot(HaveOccurred())
			b, err := io.ReadAll(reader)
			g.Expect(err).ToNot(HaveOccurred())

			gitTag := &Tag{
				Signature: tagObj.PGPSignature,
				Encoded:   b,
			}

			// Prepare authorized keys
			var authorizedKeys []string
			if tt.authorizedKeys != "" {
				authorizedKey, err := os.ReadFile(filepath.Join(testDataDir, tt.authorizedKeys))
				g.Expect(err).ToNot(HaveOccurred())
				authorizedKeys = append(authorizedKeys, string(authorizedKey))
			}

			// Verify the signature using the git.Tag's VerifySSH method
			fingerprint, err := gitTag.VerifySSH(authorizedKeys...)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				g.Expect(fingerprint).To(BeEmpty())
				return
			}

			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(fingerprint).ToNot(BeEmpty())
		})
	}
}

func TestErrRepositoryNotFound_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ErrRepositoryNotFound
		want string
	}{
		{
			name: "with message and URL",
			err: ErrRepositoryNotFound{
				Message: "repository not found",
				URL:     "https://github.com/example/repo.git",
			},
			want: "repository not found: git repository: 'https://github.com/example/repo.git'",
		},
		{
			name: "with empty message",
			err: ErrRepositoryNotFound{
				Message: "",
				URL:     "https://github.com/example/repo.git",
			},
			want: ": git repository: 'https://github.com/example/repo.git'",
		},
		{
			name: "with empty URL",
			err: ErrRepositoryNotFound{
				Message: "repository not found",
				URL:     "",
			},
			want: "repository not found: git repository: ''",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.err.Error()).To(Equal(tt.want))
		})
	}
}
