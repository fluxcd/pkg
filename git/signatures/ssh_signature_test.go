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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/git/gogit"
	"github.com/fluxcd/pkg/git/signatures"
	"github.com/fluxcd/pkg/git/testutils"
	"github.com/go-git/go-git/v5/plumbing"
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
			fingerprint, err := signatures.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, string(authorizedKey))
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
			fingerprint, err = signatures.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, string(pubKeysAll))
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
			fingerprint, err = signatures.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, string(authorizedKey))
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
			fingerprint, err = signatures.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, string(pubKeysAll))
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

	key_type := "ed25519"

	pubKey, err := os.ReadFile(filepath.Join(testDataDir, "key_"+key_type+".pub"))
	if err != nil {
		t.Fatalf("Failed to read authorized keys: %v", err)
	}

	// Parse the commit from the fixture file
	commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, "commit_"+key_type+"_signed.txt"))
	if err != nil {
		t.Fatalf("Failed to parse commit from fixture: %v", err)
	}

	// Parse the tag from the fixture file
	tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, "tag_"+key_type+"_signed.txt"))
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

		fingerprint, err := signatures.VerifySSHSignature("", gitCommit.Encoded, string(pubKey))
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for empty signature, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for empty signature: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify payload as the provided signature is empty" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify payload as the provided signature is empty'", err)
		}

		fingerprint, err = signatures.VerifySSHSignature("", gitCommit.Encoded, string(pubKey))
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

		fingerprint, err := signatures.VerifySSHSignature(gitCommit.Signature, []byte{}, string(pubKey))
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for empty payload, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for empty payload: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify payload as the provided payload is empty" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify payload as the provided payload is empty'", err)
		}

		fingerprint, err = signatures.VerifySSHSignature(gitTag.Signature, []byte{}, string(pubKey))
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

		fingerprint, err := signatures.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, wrongKey)
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

		fingerprint, err = signatures.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, wrongKey)
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

		fingerprint, err := signatures.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, emptyAuthKeys)
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for empty authorized keys, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for empty authorized keys: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify payload with any of the given authorized keys" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify payload with any of the given authorized keys'", err)
		}

		fingerprint, err = signatures.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, emptyAuthKeys)
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

		fingerprint, err := signatures.VerifySSHSignature(invalidSig, gitCommit.Encoded, string(pubKey))
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for invalid signature, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for invalid signature: %s", fingerprint)
		}
		if err != nil && !strings.Contains(err.Error(), "unable to unarmor SSH signature") {
			t.Errorf("VerifySSHSignature() error = %v, want error containing 'unable to unarmor SSH signature'", err)
		}

		fingerprint, err = signatures.VerifySSHSignature(invalidSig, gitTag.Encoded, string(pubKey))
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

		fingerprint, err := signatures.VerifySSHSignature(pgpSig, gitCommit.Encoded, "")
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for non-SSH signature, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for non-SSH signature: %s", fingerprint)
		}
		if err != nil && err.Error() != "unable to verify SSH signature, detected signature format: openpgp" {
			t.Errorf("VerifySSHSignature() error = %v, want 'unable to verify SSH signature, detected signature format: openpgp'", err)
		}

		fingerprint, err = signatures.VerifySSHSignature(pgpSig, gitTag.Encoded, "")
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

		fingerprint, err := signatures.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, invalidAuthKeys)
		if err == nil {
			t.Errorf("VerifySSHSignature() expected error for invalid authorized keys, got nil")
		}
		if fingerprint != "" {
			t.Errorf("VerifySSHSignature() returned fingerprint for invalid authorized keys: %s", fingerprint)
		}
		if err != nil && !strings.Contains(err.Error(), "unable to parse authorized key") {
			t.Errorf("VerifySSHSignature() error = %v, want error containing 'unable to parse authorized key'", err)
		}

		fingerprint, err = signatures.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, invalidAuthKeys)
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
