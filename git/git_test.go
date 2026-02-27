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
	"testing"
	"time"

	. "github.com/onsi/gomega"
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

func TestCommit_IsPGPSigned(t *testing.T) {
	tests := []struct {
		name   string
		commit *Commit
		want   bool
	}{
		{
			name: "PGP signed commit with SIGNATURE prefix",
			commit: &Commit{
				Signature: "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			},
			want: true,
		},
		{
			name: "PGP signed commit with MESSAGE prefix",
			commit: &Commit{
				Signature: "-----BEGIN PGP MESSAGE-----\n-----END PGP MESSAGE-----",
			},
			want: true,
		},
		{
			name: "SSH signed commit",
			commit: &Commit{
				Signature: "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			},
			want: false,
		},
		{
			name: "X509 signed commit",
			commit: &Commit{
				Signature: "-----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----",
			},
			want: false,
		},
		{
			name:   "unsigned commit",
			commit: &Commit{},
			want:   false,
		},
		{
			name: "PGP signed commit with leading whitespace",
			commit: &Commit{
				Signature: "  -----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.commit.IsPGPSigned()).To(Equal(tt.want))
		})
	}
}

func TestCommit_IsSSHSigned(t *testing.T) {
	tests := []struct {
		name   string
		commit *Commit
		want   bool
	}{
		{
			name: "SSH signed commit",
			commit: &Commit{
				Signature: "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			},
			want: true,
		},
		{
			name: "PGP signed commit",
			commit: &Commit{
				Signature: "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			},
			want: false,
		},
		{
			name: "X509 signed commit",
			commit: &Commit{
				Signature: "-----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----",
			},
			want: false,
		},
		{
			name:   "unsigned commit",
			commit: &Commit{},
			want:   false,
		},
		{
			name: "SSH signed commit with leading whitespace",
			commit: &Commit{
				Signature: "  -----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.commit.IsSSHSigned()).To(Equal(tt.want))
		})
	}
}

func TestCommit_SignatureType(t *testing.T) {
	tests := []struct {
		name   string
		commit *Commit
		want   string
	}{
		{
			name: "PGP signed commit with SIGNATURE prefix",
			commit: &Commit{
				Signature: "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			},
			want: "openpgp",
		},
		{
			name: "PGP signed commit with MESSAGE prefix",
			commit: &Commit{
				Signature: "-----BEGIN PGP MESSAGE-----\n-----END PGP MESSAGE-----",
			},
			want: "openpgp",
		},
		{
			name: "SSH signed commit",
			commit: &Commit{
				Signature: "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			},
			want: "ssh",
		},
		{
			name: "X509 signed commit",
			commit: &Commit{
				Signature: "-----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----",
			},
			want: "x509",
		},
		{
			name:   "unsigned commit",
			commit: &Commit{},
			want:   "unknown",
		},
		{
			name: "unknown signature type",
			commit: &Commit{
				Signature: "-----BEGIN UNKNOWN SIGNATURE-----\n-----END UNKNOWN SIGNATURE-----",
			},
			want: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.commit.SignatureType()).To(Equal(tt.want))
		})
	}
}

func TestTag_IsPGPSigned(t *testing.T) {
	tests := []struct {
		name string
		tag  *Tag
		want bool
	}{
		{
			name: "PGP signed tag with SIGNATURE prefix",
			tag: &Tag{
				Signature: "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			},
			want: true,
		},
		{
			name: "PGP signed tag with MESSAGE prefix",
			tag: &Tag{
				Signature: "-----BEGIN PGP MESSAGE-----\n-----END PGP MESSAGE-----",
			},
			want: true,
		},
		{
			name: "SSH signed tag",
			tag: &Tag{
				Signature: "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			},
			want: false,
		},
		{
			name: "X509 signed tag",
			tag: &Tag{
				Signature: "-----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----",
			},
			want: false,
		},
		{
			name: "unsigned tag",
			tag:  &Tag{},
			want: false,
		},
		{
			name: "PGP signed tag with leading whitespace",
			tag: &Tag{
				Signature: "  -----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.tag.IsPGPSigned()).To(Equal(tt.want))
		})
	}
}

func TestTag_IsSSHSigned(t *testing.T) {
	tests := []struct {
		name string
		tag  *Tag
		want bool
	}{
		{
			name: "SSH signed tag",
			tag: &Tag{
				Signature: "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			},
			want: true,
		},
		{
			name: "PGP signed tag",
			tag: &Tag{
				Signature: "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			},
			want: false,
		},
		{
			name: "X509 signed tag",
			tag: &Tag{
				Signature: "-----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----",
			},
			want: false,
		},
		{
			name: "unsigned tag",
			tag:  &Tag{},
			want: false,
		},
		{
			name: "SSH signed tag with leading whitespace",
			tag: &Tag{
				Signature: "  -----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.tag.IsSSHSigned()).To(Equal(tt.want))
		})
	}
}

func TestTag_SignatureType(t *testing.T) {
	tests := []struct {
		name string
		tag  *Tag
		want string
	}{
		{
			name: "PGP signed tag with SIGNATURE prefix",
			tag: &Tag{
				Signature: "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			},
			want: "openpgp",
		},
		{
			name: "PGP signed tag with MESSAGE prefix",
			tag: &Tag{
				Signature: "-----BEGIN PGP MESSAGE-----\n-----END PGP MESSAGE-----",
			},
			want: "openpgp",
		},
		{
			name: "SSH signed tag",
			tag: &Tag{
				Signature: "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			},
			want: "ssh",
		},
		{
			name: "X509 signed tag",
			tag: &Tag{
				Signature: "-----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----",
			},
			want: "x509",
		},
		{
			name: "unsigned tag",
			tag:  &Tag{},
			want: "unknown",
		},
		{
			name: "unknown signature type",
			tag: &Tag{
				Signature: "-----BEGIN UNKNOWN SIGNATURE-----\n-----END UNKNOWN SIGNATURE-----",
			},
			want: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(tt.tag.SignatureType()).To(Equal(tt.want))
		})
	}
}
