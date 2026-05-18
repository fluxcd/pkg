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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/signature"
	"github.com/fluxcd/pkg/git/testutils"
	"github.com/go-git/go-git/v5/plumbing"
	gossh "golang.org/x/crypto/ssh"
)

// these tests are in a different package to avoid circular dependencies with gogit.BuildCommitWithRef and gogit.BuildTag

func TestVerifySSHSignature(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	pubKeysAll, err := os.ReadFile(filepath.Join(testDataDir, "keys_all.pub"))
	if err != nil {
		t.Fatalf("Failed to read combined authorized keys: %v", err)
	}

	// Test cases for each key type using fixtures
	keyTypes := []struct {
		name             string
		signedCommitFile string
		signedTagFile    string
		pubKeyFile       string
		fingerPrintFile  string
	}{
		{
			name:             "ed25519 valid signature",
			signedCommitFile: "commit_ed25519_signed.txt",
			signedTagFile:    "tag_ed25519_signed.txt",
			pubKeyFile:       "key_ed25519.pub",
			fingerPrintFile:  "key_ed25519.pub_fingerprint",
		},
		{
			name:             "rsa valid signature",
			signedCommitFile: "commit_rsa_signed.txt",
			signedTagFile:    "tag_rsa_signed.txt",
			pubKeyFile:       "key_rsa.pub",
			fingerPrintFile:  "key_rsa.pub_fingerprint",
		},
		{
			name:             "ecdsa_p256 valid signature",
			signedCommitFile: "commit_ecdsa_p256_signed.txt",
			signedTagFile:    "tag_ecdsa_p256_signed.txt",
			pubKeyFile:       "key_ecdsa_p256.pub",
			fingerPrintFile:  "key_ecdsa_p256.pub_fingerprint",
		},
		{
			name:             "ecdsa_p384 valid signature",
			signedCommitFile: "commit_ecdsa_p384_signed.txt",
			signedTagFile:    "tag_ecdsa_p384_signed.txt",
			pubKeyFile:       "key_ecdsa_p384.pub",
			fingerPrintFile:  "key_ecdsa_p384.pub_fingerprint",
		},
		{
			name:             "ecdsa_p521 valid signature",
			signedCommitFile: "commit_ecdsa_p521_signed.txt",
			signedTagFile:    "tag_ecdsa_p521_signed.txt",
			pubKeyFile:       "key_ecdsa_p521.pub",
			fingerPrintFile:  "key_ecdsa_p521.pub_fingerprint",
		},
	}

	for _, kt := range keyTypes {
		t.Run(kt.name, func(t *testing.T) {

			// Parse the commit from the fixture file
			commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, kt.signedCommitFile))
			if err != nil {
				t.Fatalf("Failed to parse commit from fixture: %v", err)
			}

			// Build a git.Commit using BuildCommitWithRef
			gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
			if err != nil {
				t.Fatalf("Failed to build commit: %v", err)
			}

			// Parse the commit from the fixture file
			tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, kt.signedTagFile))
			if err != nil {
				t.Fatalf("Failed to parse commit from fixture: %v", err)
			}

			// Build a git.Commit using BuildCommitWithRef
			gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
			if err != nil {
				t.Fatalf("Failed to build commit: %v", err)
			}

			// Read the authorized keys
			authorizedKey, err := os.ReadFile(filepath.Join(testDataDir, kt.pubKeyFile))
			if err != nil {
				t.Fatalf("Failed to read authorized keys: %v", err)
			}

			expectedFingerprintBytes, err := os.ReadFile(filepath.Join(testDataDir, kt.fingerPrintFile))
			if err != nil {
				t.Fatalf("Failed to read fingerprint file %s: %v", kt.fingerPrintFile, err)
			}
			expectedFingerprint := strings.TrimSpace(string(expectedFingerprintBytes))

			// Verify the signature using the git.Commit's Signature and Encoded fields
			fingerprint, err := signature.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, string(authorizedKey))
			if err != nil {
				t.Errorf("Commit signature VerifySSHSignature() error = %v", err)
			}
			if fingerprint == "" {
				t.Errorf("Commit signature VerifySSHSignature() returned empty fingerprint")
			}
			if fingerprint != expectedFingerprint {
				t.Errorf("Commit signature VerifySSHSignature() fingerprint mismatch, got '%s', want '%s'", fingerprint, expectedFingerprint)
			}

			// Verifying the correct fingerprint is returned from a list of public keys
			fingerprint, err = signature.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, string(pubKeysAll))
			if err != nil {
				t.Errorf("Commit signature VerifySSHSignature() error = %v", err)
			}
			if fingerprint == "" {
				t.Errorf("Commit signature VerifySSHSignature() returned empty fingerprint")
			}
			if fingerprint != expectedFingerprint {
				t.Errorf("Commit signature VerifySSHSignature() fingerprint mismatch, got '%s', want '%s'", fingerprint, expectedFingerprint)
			}

			// Verify the signature using the git.Tag's Signature and Encoded fields
			fingerprint, err = signature.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, string(authorizedKey))
			if err != nil {
				t.Errorf("Tag signature VerifySSHSignature() error = %v", err)
			}
			if fingerprint == "" {
				t.Errorf("Tag signature VerifySSHSignature() returned empty fingerprint")
			}
			if fingerprint != expectedFingerprint {
				t.Errorf("Tag signature VerifySSHSignature() fingerprint mismatch, got '%s', want '%s'", fingerprint, expectedFingerprint)
			}

			// Verifying the correct fingerprint is returned from a list of public keys
			fingerprint, err = signature.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, string(pubKeysAll))
			if err != nil {
				t.Errorf("Tag signature VerifySSHSignature() error = %v", err)
			}
			if fingerprint == "" {
				t.Errorf("Tag signature VerifySSHSignature() returned empty fingerprint")
			}
			if fingerprint != expectedFingerprint {
				t.Errorf("Tag signature VerifySSHSignature() fingerprint mismatch, got '%s', want '%s'", fingerprint, expectedFingerprint)
			}

		})
	}
}

func TestSSHSignatureValidationCases(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	keyType := "ed25519"

	pubKey, err := os.ReadFile(filepath.Join(testDataDir, "key_"+keyType+".pub"))
	if err != nil {
		t.Fatalf("Failed to read authorized keys: %v", err)
	}

	// Parse the commit from the fixture file
	commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, "commit_"+keyType+"_signed.txt"))
	if err != nil {
		t.Fatalf("Failed to parse commit from fixture: %v", err)
	}

	// Parse the tag from the fixture file
	tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, "tag_"+keyType+"_signed.txt"))
	if err != nil {
		t.Fatalf("Failed to parse tag from fixture: %v", err)
	}

	// Build a git.Commit using BuildCommitWithRef
	gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
	if err != nil {
		t.Fatalf("Failed to build commit: %v", err)
	}

	// Build a git.Tag using BuildTag
	gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
	if err != nil {
		t.Fatalf("Failed to build tag: %v", err)
	}

	// Test error cases
	t.Run("empty signature", func(t *testing.T) {

		fingerprint, err := signature.VerifySSHSignature("", gitCommit.Encoded, string(pubKey))
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for empty signature, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for empty signature: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify payload as the provided signature is empty" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify payload as the provided signature is empty'", err)
		}

		fingerprint, err = signature.VerifySSHSignature("", gitCommit.Encoded, string(pubKey))
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for empty signature, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for empty signature: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify payload as the provided signature is empty" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify payload as the provided signature is empty'", err)
		}

	})

	t.Run("empty payload", func(t *testing.T) {

		fingerprint, err := signature.VerifySSHSignature(gitCommit.Signature, []byte{}, string(pubKey))
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for empty payload, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for empty payload: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify payload as the provided payload is empty" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify payload as the provided payload is empty'", err)
		}

		fingerprint, err = signature.VerifySSHSignature(gitTag.Signature, []byte{}, string(pubKey))
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for empty payload, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for empty payload: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify payload as the provided payload is empty" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify payload as the provided payload is empty'", err)
		}

	})

	t.Run("wrong authorized keys", func(t *testing.T) {
		// Use a different key that won't match
		wrongKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEyM97VxLgOCuB9Eg5cDtTc8ogkdM1xAyJhzODB9cK1 wrong@example.com"

		fingerprint, err := signature.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, wrongKey)
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for wrong authorized keys, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for wrong authorized keys: %s", fingerprint)
		}
		// The error can be either a parsing error or a verification error
		if err != nil && !strings.Contains(err.Error(), "unable to verify payload with any of the given authorized keys") && !strings.Contains(err.Error(), "unable to parse authorized key") {
			t.Errorf("VerifySSHSignature() error = %v, want error containing 'unable to verify payload with any of the given authorized keys' or 'unable to parse authorized key'", err)
		}

		fingerprint, err = signature.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, wrongKey)
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for wrong authorized keys, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for wrong authorized keys: %s", fingerprint)
		}
		// The error can be either a parsing error or a verification error
		if err != nil && !strings.Contains(err.Error(), "unable to verify payload with any of the given authorized keys") && !strings.Contains(err.Error(), "unable to parse authorized key") {
			t.Errorf("VerifySSHSignature() error = %v, want error containing 'unable to verify payload with any of the given authorized keys' or 'unable to parse authorized key'", err)
		}
	})

	t.Run("empty authorized keys", func(t *testing.T) {
		// Use empty authorized keys
		emptyAuthKeys := ""

		fingerprint, err := signature.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, emptyAuthKeys)
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for empty authorized keys, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for empty authorized keys: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify payload with any of the given authorized keys" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify payload with any of the given authorized keys'", err)
		}

		fingerprint, err = signature.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, emptyAuthKeys)
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for empty authorized keys, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for empty authorized keys: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify payload with any of the given authorized keys" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify payload with any of the given authorized keys'", err)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		invalidSig := "-----BEGIN SSH SIGNATURE-----\n invalid\n -----END SSH SIGNATURE-----"

		fingerprint, err := signature.VerifySSHSignature(invalidSig, gitCommit.Encoded, string(pubKey))
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for invalid signature, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for invalid signature: %s", fingerprint)
		}
		if err != nil && !strings.Contains(err.Error(), "unable to unarmor SSH signature") {
			t.Errorf("VerifySSHSignature() error = %v, want error containing 'unable to unarmor SSH signature'", err)
		}

		fingerprint, err = signature.VerifySSHSignature(invalidSig, gitTag.Encoded, string(pubKey))
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for invalid signature, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for invalid signature: %s", fingerprint)
		}
		if err != nil && !strings.Contains(err.Error(), "unable to unarmor SSH signature") {
			t.Errorf("VerifySSHSignature() error = %v, want error containing 'unable to unarmor SSH signature'", err)
		}

	})

	t.Run("non-SSH signature", func(t *testing.T) {
		// Use a PGP signature instead of SSH signature
		pgpSig := "-----BEGIN PGP SIGNATURE-----\n-----END PGP SIGNATURE-----"

		fingerprint, err := signature.VerifySSHSignature(pgpSig, gitCommit.Encoded, "")
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for non-SSH signature, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for non-SSH signature: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify SSH signature, detected signature format: openpgp" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify SSH signature, detected signature format: openpgp'", err)
		}

		fingerprint, err = signature.VerifySSHSignature(pgpSig, gitTag.Encoded, "")
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for non-SSH signature, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for non-SSH signature: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify SSH signature, detected signature format: openpgp" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify SSH signature, detected signature format: openpgp'", err)
		}
	})

	t.Run("invalid authorized keys", func(t *testing.T) {
		// Use invalid authorized keys
		invalidAuthKeys := "invalid-key-data"

		fingerprint, err := signature.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, invalidAuthKeys)
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for invalid authorized keys, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for invalid authorized keys: %s", fingerprint)
		}
		if err != nil && !strings.Contains(err.Error(), "unable to parse authorized key") {
			t.Errorf("VerifySSHSignature() error = %v, want error containing 'unable to parse authorized key'", err)
		}

		fingerprint, err = signature.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, invalidAuthKeys)
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for invalid authorized keys, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for invalid authorized keys: %s", fingerprint)
		}
		if err != nil && !strings.Contains(err.Error(), "unable to parse authorized key") {
			t.Errorf("VerifySSHSignature() error = %v, want error containing 'unable to parse authorized key'", err)
		}
	})
}

func TestParseAuthorizedKeysAndPublicFingerprint(t *testing.T) {
	tests := []struct {
		name             string
		authorizedKeys   string
		wantCount        int
		wantErr          bool
		wantFingerprints []string
	}{
		{
			name:             "single key",
			authorizedKeys:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPbmoVMAS5Ttg77s9DLSAOf4gXCiQpgdRekFHlzbXHLH test@example.com",
			wantCount:        1,
			wantErr:          false,
			wantFingerprints: []string{"SHA256:CGIPzdGcFuLkjItmqTm5kJNvof4yB662MxZXoxntLYM"},
		},
		{
			name:             "key with additional directives",
			authorizedKeys:   "no-user-rc,no-agent-forwarding ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPbmoVMAS5Ttg77s9DLSAOf4gXCiQpgdRekFHlzbXHLH test@example.com additional long comment about nothing",
			wantCount:        1,
			wantErr:          false,
			wantFingerprints: []string{"SHA256:CGIPzdGcFuLkjItmqTm5kJNvof4yB662MxZXoxntLYM"},
		},
		{
			name: "multiple keys",
			authorizedKeys: `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPbmoVMAS5Ttg77s9DLSAOf4gXCiQpgdRekFHlzbXHLH test1@example.com
ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBL7Xspf5BmRD7ipGo4SNCftjzeunry1znmU78RhcVOYwLNCR5MVm22N9c1aYacIxHmi/TxkNTdQdEB8dd4mfA4Q= test-ecdsa_p256@example.com`,
			wantCount:        2,
			wantErr:          false,
			wantFingerprints: []string{"SHA256:CGIPzdGcFuLkjItmqTm5kJNvof4yB662MxZXoxntLYM", "SHA256:oU8IT7UOnJlOTOvr/W1cYf1SkdocFm5F7SAXOwuo8Kc"},
		},
		{
			name: "with comments",
			authorizedKeys: `# This is a comment
ecdsa-sha2-nistp384 AAAAE2VjZHNhLXNoYTItbmlzdHAzODQAAAAIbmlzdHAzODQAAABhBKQ9Upb3Pa7b5NWbozm20PqpFc5WZCCCBlX9+eFELAjKdBze2EbTTKvx9YskKJ8PWLE8D9w20sjDivNwfUjoiZGgbJQcJKcKPrtovOYPv0JKpoyZ0PuLpq9kjSRTRnShEw== test-ecdsa_p384@example.com
# Another comment`,
			wantCount:        1,
			wantErr:          false,
			wantFingerprints: []string{"SHA256:+vwrYGpHfAAWIzT2x+uV+duJG7ZnSvCbRKwdPApx7JA"},
		},
		{
			name: "with empty lines",
			authorizedKeys: `ecdsa-sha2-nistp521 AAAAE2VjZHNhLXNoYTItbmlzdHA1MjEAAAAIbmlzdHA1MjEAAACFBAGSY+OAEbrNSJ4QD6NgJIJQV8kmjqi+BhfeAAthEv0eCq1CADrCqKt0poxCahYNCTMLlMvqW7xBw6wDB0kV0/4CTwBX9HRftFUpaZanPtfvMNhPT/CDMrTsNSzg/H32Hu/fuvLwyPQ0JzRXgf+qiq3OZ4q0VjERU7L13UDoz4FgHJIeVQ== test-ecdsa_p521@example.com

ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQCpreiO+8XsB4xXGNmwuO48a7WPghb5ihCJNPyQZpnaPfq6vhNVWSgq8AIjBmJOJYo4HZyiHqpS4OBc86glk6qMv8YHRt4VRVBP+DjPLDIsOR7+2HBlOPHMm8lTDi+iMPHBDxqFy7mSDB4+v7n700+49vYhWjZJpesnnE6JoitxSVhmqp75jeNRNU6PD00z+gMUcviv8UOs/Apg1Cw5f+4T9yOnjlOHaFH/ButvZ0t2VF0cs28tfCuLAoumjine5Gm6tCRQlZOoapNJzvnYT+86f/PEU/4kDYf3wT7S+NnUDfCsIpDVlOXPvjnQ/DudhqEnnXvfch+eBCI7rtJBHIGPKFdmC4cUROa0UDGR6o/JxLtx4ZTbkGpq6MVwdrb7qJ+Oib1U8xVimWFfarkm7deVXWD3wB5Wa8Ko/a/WuYfE3gYRhb8iXPYd71FsEy4F41JCMZDcIqMiQRe3e2gvY+z2sf02kHOFeWJmrAY9FFjPL85VD0Dg++jrExkGFjcBTw9gUG5OPGpwqQ9WHO8E8DPza+i5J/wu4DODyLrLxuXHPeSYUjcvh5ln8P70qL+Irwn1mgn2PkIZW0XCPBt6Iylg55t5sfyy03P0Kmb4U3TrppMeig7Lr9LDU4Doh7Fj6oLYDGFUV+F52SSuPs5SfrWd6Apiz+VPjsAh5btPPJNlzQ== test-rsa@example.com`,
			wantCount:        2,
			wantErr:          false,
			wantFingerprints: []string{"SHA256:3FcWgX5RsACruglrcBJP/hefUZcYHJGnrk07U6yKin8", "SHA256:TxoYgaeIj5A7Md4rHNfxPdqawooc4NIGjIMbcQ7YKbw"},
		},
		{
			name:             "empty",
			authorizedKeys:   "",
			wantCount:        0,
			wantErr:          false,
			wantFingerprints: []string{},
		},
		{
			name:             "invalid key",
			authorizedKeys:   "invalid-key-data",
			wantCount:        0,
			wantErr:          true,
			wantFingerprints: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := signature.ParseAuthorizedKeys(tt.authorizedKeys)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAuthorizedKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(keys) != tt.wantCount {
				t.Errorf("ParseAuthorizedKeys() got %d keys, want %d", len(keys), tt.wantCount)
			}
			// Validate expected fingerprint if specified
			if len(tt.wantFingerprints) > 0 && len(keys) > 0 {
				for _, key := range keys {
					found := false
					fingerprint := gossh.FingerprintSHA256(key)
					for _, wantedFingerprint := range tt.wantFingerprints {
						if fingerprint == wantedFingerprint {
							found = true
						}
					}
					if !found {
						t.Errorf("ParseAuthorizedKeys() fingerprint '%s'not in list of wanted fingerprints %s", fingerprint, tt.wantFingerprints)
					}
				}
			}
		})
	}
}

func TestParseAuthorizedKeysFromFixtures(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	tests := []struct {
		name            string
		fixture         string
		fingerprintFile string
		wantCount       int
		wantErr         bool
	}{
		{
			name:            "ed25519 key",
			fixture:         "key_ed25519.pub",
			fingerprintFile: "key_ed25519.pub_fingerprint",
			wantCount:       1,
			wantErr:         false,
		},
		{
			name:            "rsa key",
			fixture:         "key_rsa.pub",
			fingerprintFile: "key_rsa.pub_fingerprint",
			wantCount:       1,
			wantErr:         false,
		},
		{
			name:            "ecdsa p256 key",
			fixture:         "key_ecdsa_p256.pub",
			fingerprintFile: "key_ecdsa_p256.pub_fingerprint",
			wantCount:       1,
			wantErr:         false,
		},
		{
			name:            "ecdsa p384 key",
			fixture:         "key_ecdsa_p384.pub",
			fingerprintFile: "key_ecdsa_p384.pub_fingerprint",
			wantCount:       1,
			wantErr:         false,
		},
		{
			name:            "ecdsa p521 key",
			fixture:         "key_ecdsa_p521.pub",
			fingerprintFile: "key_ecdsa_p521.pub_fingerprint",
			wantCount:       1,
			wantErr:         false,
		},
		{
			name:      "all key types combined",
			fixture:   "keys_all.pub",
			wantCount: 5,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, tt.fixture))
			if err != nil {
				t.Fatalf("Failed to read fixture file %s: %v", tt.fixture, err)
			}

			keys, err := signature.ParseAuthorizedKeys(string(authorizedKeys))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAuthorizedKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(keys) != tt.wantCount {
				t.Errorf("ParseAuthorizedKeys() got %d keys, want %d", len(keys), tt.wantCount)
			}

			// Read expected fingerprint from file if provided
			var expectedFingerprint string
			if tt.fingerprintFile != "" {
				fingerprintData, err := os.ReadFile(filepath.Join(testDataDir, tt.fingerprintFile))
				if err != nil {
					t.Fatalf("Failed to read fingerprint file %s: %v", tt.fingerprintFile, err)
				}
				expectedFingerprint = strings.TrimSpace(string(fingerprintData))
			}

			// Verify that each key has a valid fingerprint
			for i, key := range keys {
				fingerprint := gossh.FingerprintSHA256(key)
				if fingerprint == "" {
					t.Errorf("Key %d has empty fingerprint", i)
				}
				if !strings.HasPrefix(fingerprint, "SHA256:") {
					t.Errorf("Key %d fingerprint %s does not have SHA256: prefix", i, fingerprint)
				}
				// Validate fingerprint against the one read from file
				if expectedFingerprint != "" {
					if fingerprint != expectedFingerprint {
						t.Errorf("Key %d got fingerprint %s, want %s (from %s)", i, fingerprint, expectedFingerprint, tt.fingerprintFile)
					}
				}
			}
		})
	}
}

func TestParseAuthorizedKeysCombinations(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	tests := []struct {
		name      string
		fixtures  []string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "ed25519 + rsa",
			fixtures:  []string{"key_ed25519.pub", "key_rsa.pub"},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "ed25519 + ecdsa p256",
			fixtures:  []string{"key_ed25519.pub", "key_ecdsa_p256.pub"},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "rsa + ecdsa p384 + ecdsa p521",
			fixtures:  []string{"key_rsa.pub", "key_ecdsa_p384.pub", "key_ecdsa_p521.pub"},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "all ecdsa variants",
			fixtures:  []string{"key_ecdsa_p256.pub", "key_ecdsa_p384.pub", "key_ecdsa_p521.pub"},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "ed25519 + rsa + all ecdsa",
			fixtures:  []string{"key_ed25519.pub", "key_rsa.pub", "key_ecdsa_p256.pub", "key_ecdsa_p384.pub", "key_ecdsa_p521.pub"},
			wantCount: 5,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var combinedKeys strings.Builder
			for _, fixture := range tt.fixtures {
				authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, fixture))
				if err != nil {
					t.Fatalf("Failed to read fixture file %s: %v", fixture, err)
				}
				combinedKeys.Write(authorizedKeys)
				combinedKeys.WriteString("\n")
			}

			keys, err := signature.ParseAuthorizedKeys(combinedKeys.String())
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAuthorizedKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(keys) != tt.wantCount {
				t.Errorf("ParseAuthorizedKeys() got %d keys, want %d", len(keys), tt.wantCount)
			}

			// Verify that each key has a valid fingerprint
			for i, key := range keys {
				fingerprint := gossh.FingerprintSHA256(key)
				if fingerprint == "" {
					t.Errorf("Key %d has empty fingerprint", i)
				}
				if !strings.HasPrefix(fingerprint, "SHA256:") {
					t.Errorf("Key %d fingerprint %s does not have SHA256: prefix", i, fingerprint)
				}
			}
		})
	}
}
