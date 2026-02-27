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

package signatures_test

import (
	"testing"

	. "github.com/fluxcd/pkg/git/signatures"
)

func TestIsPGPSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		want      bool
	}{
		{
			name:      "valid PGP signature",
			signature: "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			want:      true,
		},
		{
			name:      "PGP signature with leading whitespace",
			signature: "  -----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			want:      true,
		},
		{
			name:      "valid PGP signature",
			signature: "-----BEGIN PGP MESSAGE-----\n-----END PGP MESSAGE-----",
			want:      true,
		},
		{
			name:      "PGP signature with leading whitespace",
			signature: "  -----BEGIN PGP MESSAGE-----\n-----END PGP MESSAGE-----",
			want:      true,
		},
		{
			name:      "empty signature",
			signature: "",
			want:      false,
		},
		{
			name:      "SSH signature",
			signature: "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			want:      false,
		},
		{
			name:      "unknown signature",
			signature: "-----BEGIN UNKNOWN SIGNATURE-----\n-----END UNKNOWN SIGNATURE-----",
			want:      false,
		},
		{
			name:      "whitespace only",
			signature: "   \n\t  ",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPGPSignature(tt.signature); got != tt.want {
				t.Errorf("IsPGPSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSSHSignature(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		want      bool
	}{
		{
			name:      "valid SSH signature",
			signature: "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			want:      true,
		},
		{
			name:      "SSH signature with leading whitespace",
			signature: "  -----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			want:      true,
		},
		{
			name:      "empty signature",
			signature: "",
			want:      false,
		},
		{
			name:      "PGP signature",
			signature: "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			want:      false,
		},
		{
			name:      "unknown signature",
			signature: "-----BEGIN UNKNOWN SIGNATURE-----\n-----END UNKNOWN SIGNATURE-----",
			want:      false,
		},
		{
			name:      "whitespace only",
			signature: "   \n\t  ",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSSHSignature(tt.signature); got != tt.want {
				t.Errorf("IsSSHSignature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsx509Signature(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		want      bool
	}{
		{
			name:      "valid x509 signature",
			signature: "-----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----",
			want:      true,
		},
		{
			name:      "x509 signature with leading whitespace",
			signature: "  -----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----",
			want:      true,
		},
		{
			name:      "empty signature",
			signature: "",
			want:      false,
		},
		{
			name:      "PGP signature",
			signature: "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			want:      false,
		},
		{
			name:      "SSH signature",
			signature: "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			want:      false,
		},
		{
			name:      "unknown signature",
			signature: "-----BEGIN UNKNOWN SIGNATURE-----\n-----END UNKNOWN SIGNATURE-----",
			want:      false,
		},
		{
			name:      "whitespace only",
			signature: "   \n\t  ",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Isx509Signature(tt.signature); got != tt.want {
				t.Errorf("Isx509Signature() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSignatureType(t *testing.T) {
	tests := []struct {
		name      string
		signature string
		want      string
	}{
		{
			name:      "PGP signature",
			signature: "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			want:      string(SignatureTypePGP),
		},
		{
			name:      "PGP signature with leading whitespace",
			signature: "  -----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----",
			want:      string(SignatureTypePGP),
		},
		{
			name:      "SSH signature",
			signature: "-----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			want:      string(SignatureTypeSSH),
		},
		{
			name:      "SSH signature with leading whitespace",
			signature: "  -----BEGIN SSH SIGNATURE-----\n-----END SSH SIGNATURE-----",
			want:      string(SignatureTypeSSH),
		},
		{
			name:      "x509 signature",
			signature: "-----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----",
			want:      string(SignatureTypeX509),
		},
		{
			name:      "x509 signature with leading whitespace",
			signature: "  -----BEGIN SIGNED MESSAGE-----\n-----END SIGNED MESSAGE-----",
			want:      string(SignatureTypeX509),
		},
		{
			name:      "empty signature",
			signature: "",
			want:      string(SignatureTypeUnknown),
		},
		{
			name:      "unknown signature",
			signature: "-----BEGIN UNKNOWN SIGNATURE-----\n-----END UNKNOWN SIGNATURE-----",
			want:      string(SignatureTypeUnknown),
		},
		{
			name:      "whitespace only",
			signature: "   \n\t  ",
			want:      string(SignatureTypeUnknown),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetSignatureType(tt.signature); got != tt.want {
				t.Errorf("GetSignatureType() = %v, want %v", got, tt.want)
			}
		})
	}
}
