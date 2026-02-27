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
	"github.com/hiddeco/sshsig"
)

func TestParseAuthorizedKeys(t *testing.T) {
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
			keys, err := signatures.ParseAuthorizedKeys(tt.authorizedKeys)
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
					fingerprint := signatures.GetPublicKeyFingerprint(key)
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
			fixture:         "authorized_keys_ed25519",
			fingerprintFile: "key_ed25519.pub_fingerprint",
			wantCount:       1,
			wantErr:         false,
		},
		{
			name:            "rsa key",
			fixture:         "authorized_keys_rsa",
			fingerprintFile: "key_rsa.pub_fingerprint",
			wantCount:       1,
			wantErr:         false,
		},
		{
			name:            "ecdsa p256 key",
			fixture:         "authorized_keys_ecdsa_p256",
			fingerprintFile: "key_ecdsa_p256.pub_fingerprint",
			wantCount:       1,
			wantErr:         false,
		},
		{
			name:            "ecdsa p384 key",
			fixture:         "authorized_keys_ecdsa_p384",
			fingerprintFile: "key_ecdsa_p384.pub_fingerprint",
			wantCount:       1,
			wantErr:         false,
		},
		{
			name:            "ecdsa p521 key",
			fixture:         "authorized_keys_ecdsa_p521",
			fingerprintFile: "key_ecdsa_p521.pub_fingerprint",
			wantCount:       1,
			wantErr:         false,
		},
		{
			name:      "all key types combined",
			fixture:   "authorized_keys_all",
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

			keys, err := signatures.ParseAuthorizedKeys(string(authorizedKeys))
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
				fingerprint := signatures.GetPublicKeyFingerprint(key)
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
			fixtures:  []string{"authorized_keys_ed25519", "authorized_keys_rsa"},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "ed25519 + ecdsa p256",
			fixtures:  []string{"authorized_keys_ed25519", "authorized_keys_ecdsa_p256"},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "rsa + ecdsa p384 + ecdsa p521",
			fixtures:  []string{"authorized_keys_rsa", "authorized_keys_ecdsa_p384", "authorized_keys_ecdsa_p521"},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "all ecdsa variants",
			fixtures:  []string{"authorized_keys_ecdsa_p256", "authorized_keys_ecdsa_p384", "authorized_keys_ecdsa_p521"},
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "ed25519 + rsa + all ecdsa",
			fixtures:  []string{"authorized_keys_ed25519", "authorized_keys_rsa", "authorized_keys_ecdsa_p256", "authorized_keys_ecdsa_p384", "authorized_keys_ecdsa_p521"},
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

			keys, err := signatures.ParseAuthorizedKeys(combinedKeys.String())
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAuthorizedKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(keys) != tt.wantCount {
				t.Errorf("ParseAuthorizedKeys() got %d keys, want %d", len(keys), tt.wantCount)
			}

			// Verify that each key has a valid fingerprint
			for i, key := range keys {
				fingerprint := signatures.GetPublicKeyFingerprint(key)
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

func TestParseSSHSignature(t *testing.T) {
	tests := []struct {
		name    string
		sig     string
		wantErr bool
	}{
		{
			name: "valid signature with PEM armor",
			sig: `-----BEGIN SSH SIGNATURE-----
U1NIU0lHAAAAAQAAADMAAAALc3NoLWVkMjU1MTkAAAAg9uahUwBLlO2Dvuz0MtIA5/iBcK
JCmB1F6QUeXNtccscAAAADZ2l0AAAAAAAAAAZzaGE1MTIAAABTAAAAC3NzaC1lZDI1NTE5
AAAAQFb88f1ZXOK1BByC4QQOthH9bZP0/hMcPl62h4oIuEny6W5xd/oOpDv7dmj9A6DiMS
o6RLdWlvb81l/UyYhGEwE=
-----END SSH SIGNATURE-----`,
			wantErr: false,
		},
		{
			name:    "valid signature without PEM armor",
			sig:     "U1NIU0lHAAAAAQAAADMAAAALc3NoLWVkMjU1MTkAAAAg9uahUwBLlO2Dvuz0MtIA5/iBcKJCmB1F6QUeXNtccscAAAADZ2l0AAAAAAAAAAZzaGE1MTIAAABTAAAAC3NzaC1lZDI1NTE5AAAAQFb88f1ZXOK1BByC4QQOthH9bZP0/hMcPl62h4oIuEny6W5xd/oOpDv7dmj9A6DiMSo6RLdWlvb81l/UyYhGEwE=",
			wantErr: true, // sshsig.Unarmor() requires PEM armor
		},
		{
			name:    "empty signature",
			sig:     "",
			wantErr: true,
		},
		{
			name:    "invalid base64",
			sig:     "-----BEGIN SSH SIGNATURE-----\ninvalid-base64!!!\n-----END SSH SIGNATURE-----",
			wantErr: true,
		},
		{
			name:    "invalid format",
			sig:     "invalid-signature-format",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig, err := sshsig.Unarmor([]byte(tt.sig))
			if (err != nil) != tt.wantErr {
				t.Errorf("sshsig.Unarmor() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && sig == nil {
				t.Errorf("sshsig.Unarmor() returned nil signature")
			}
		})
	}
}

func TestGetPublicKeyFingerprint(t *testing.T) {
	// Test with a known public key
	pubKeyStr := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIPbmoVMAS5Ttg77s9DLSAOf4gXCiQpgdRekFHlzbXHLH test@example.com"
	expectedFingerprint := "SHA256:CGIPzdGcFuLkjItmqTm5kJNvof4yB662MxZXoxntLYM"
	keys, err := signatures.ParseAuthorizedKeys(pubKeyStr)
	if err != nil {
		t.Fatalf("Failed to parse test public key: %v", err)
	}
	if len(keys) == 0 {
		t.Fatal("No keys parsed")
	}

	fingerprint := signatures.GetPublicKeyFingerprint(keys[0])
	if fingerprint == "" {
		t.Error("GetPublicKeyFingerprint() returned empty string")
	}
	if !strings.HasPrefix(fingerprint, expectedFingerprint) {
		t.Errorf("GetPublicKeyFingerprint() = %s, want prefix SHA256:", fingerprint)
	}
}

func TestVerifySSHSignature(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	// Test cases for each key type using fixtures
	keyTypes := []struct {
		name     string
		sigFile  string
		authFile string
		wantErr  bool
	}{
		{"ed25519 valid signature", "commit_ed25519_signed.txt", "authorized_keys_ed25519", false},
		{"rsa valid signature", "commit_rsa_signed.txt", "authorized_keys_rsa", false},
		{"ecdsa_p256 valid signature", "commit_ecdsa_p256_signed.txt", "authorized_keys_ecdsa_p256", false},
		{"ecdsa_p384 valid signature", "commit_ecdsa_p384_signed.txt", "authorized_keys_ecdsa_p384", false},
		{"ecdsa_p521 valid signature", "commit_ecdsa_p521_signed.txt", "authorized_keys_ecdsa_p521", false},
	}

	for _, kt := range keyTypes {
		t.Run(kt.name, func(t *testing.T) {
			// Parse the commit from the fixture file
			commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, kt.sigFile))
			if err != nil {
				t.Fatalf("Failed to parse commit from fixture: %v", err)
			}

			// Build a git.Commit using BuildCommitWithRef
			gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
			if err != nil {
				t.Fatalf("Failed to build commit: %v", err)
			}

			// Read the authorized keys
			authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, kt.authFile))
			if err != nil {
				t.Fatalf("Failed to read authorized keys: %v", err)
			}

			// Verify the signature using the git.Commit's Signature and Encoded fields
			fingerprint, err := signatures.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, string(authorizedKeys))
			if (err != nil) != kt.wantErr {
				t.Errorf("VerifySSHSignature() error = %v, wantErr %v", err, kt.wantErr)
				return
			}
			if !kt.wantErr && fingerprint == "" {
				t.Errorf("VerifySSHSignature() returned empty fingerprint")
			}
			if !kt.wantErr {
				t.Logf("Verified with fingerprint: %s", fingerprint)
			}
		})
	}

	// Test error cases
	t.Run("empty signature", func(t *testing.T) {
		authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, "authorized_keys_ed25519"))
		if err != nil {
			t.Fatalf("Failed to read authorized keys: %v", err)
		}

		// Parse the commit from the fixture file
		commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, "commit_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse commit from fixture: %v", err)
		}

		// Build a git.Commit using BuildCommitWithRef
		gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
		if err != nil {
			t.Fatalf("Failed to build commit: %v", err)
		}

		fingerprint, err := signatures.VerifySSHSignature("", gitCommit.Encoded, string(authorizedKeys))
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
		authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, "authorized_keys_ed25519"))
		if err != nil {
			t.Fatalf("Failed to read authorized keys: %v", err)
		}

		// Parse the commit from the fixture file
		commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, "commit_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse commit from fixture: %v", err)
		}

		// Build a git.Commit using BuildCommitWithRef
		gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
		if err != nil {
			t.Fatalf("Failed to build commit: %v", err)
		}

		fingerprint, err := signatures.VerifySSHSignature(gitCommit.Signature, []byte{}, string(authorizedKeys))
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
		// Parse the commit from the fixture file
		commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, "commit_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse commit from fixture: %v", err)
		}

		// Build a git.Commit using BuildCommitWithRef
		gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
		if err != nil {
			t.Fatalf("Failed to build commit: %v", err)
		}

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
	})

	t.Run("empty authorized keys", func(t *testing.T) {
		// Parse the commit from the fixture file
		commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, "commit_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse commit from fixture: %v", err)
		}

		// Build a git.Commit using BuildCommitWithRef
		gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
		if err != nil {
			t.Fatalf("Failed to build commit: %v", err)
		}

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
	})

	t.Run("invalid signature", func(t *testing.T) {
		authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, "authorized_keys_ed25519"))
		if err != nil {
			t.Fatalf("Failed to read authorized keys: %v", err)
		}

		// Parse the commit from the fixture file
		commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, "commit_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse commit from fixture: %v", err)
		}

		// Build a git.Commit using BuildCommitWithRef
		gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
		if err != nil {
			t.Fatalf("Failed to build commit: %v", err)
		}

		invalidSig := "-----BEGIN SSH SIGNATURE-----\n invalid\n -----END SSH SIGNATURE-----"

		fingerprint, err := signatures.VerifySSHSignature(invalidSig, gitCommit.Encoded, string(authorizedKeys))
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
		// Parse the commit from the fixture file
		commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, "commit_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse commit from fixture: %v", err)
		}

		// Build a git.Commit using BuildCommitWithRef
		gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
		if err != nil {
			t.Fatalf("Failed to build commit: %v", err)
		}

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
	})

	t.Run("invalid authorized keys", func(t *testing.T) {
		// Parse the commit from the fixture file
		commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, "commit_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse commit from fixture: %v", err)
		}

		// Build a git.Commit using BuildCommitWithRef
		gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
		if err != nil {
			t.Fatalf("Failed to build commit: %v", err)
		}

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
	})
}

func TestVerifySSHSignatureAllKeyTypes(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	// Test cases for each key type
	keyTypes := []struct {
		name     string
		sigFile  string
		authFile string
		wantErr  bool
	}{
		{"ed25519", "commit_ed25519_signed.txt", "authorized_keys_ed25519", false},
		{"rsa", "commit_rsa_signed.txt", "authorized_keys_rsa", false},
		{"ecdsa_p256", "commit_ecdsa_p256_signed.txt", "authorized_keys_ecdsa_p256", false},
		{"ecdsa_p384", "commit_ecdsa_p384_signed.txt", "authorized_keys_ecdsa_p384", false},
		{"ecdsa_p521", "commit_ecdsa_p521_signed.txt", "authorized_keys_ecdsa_p521", false},
	}

	for _, kt := range keyTypes {
		t.Run(kt.name, func(t *testing.T) {
			// Parse the commit from the fixture file
			commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, kt.sigFile))
			if err != nil {
				t.Fatalf("Failed to parse commit from fixture: %v", err)
			}

			// Build a git.Commit using BuildCommitWithRef
			gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
			if err != nil {
				t.Fatalf("Failed to build commit: %v", err)
			}

			// Read the authorized keys
			authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, kt.authFile))
			if err != nil {
				t.Fatalf("Failed to read authorized keys: %v", err)
			}

			// Verify the signature using the git.Commit's Signature and Encoded fields
			fingerprint, err := signatures.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, string(authorizedKeys))
			if (err != nil) != kt.wantErr {
				t.Errorf("VerifySSHSignature() error = %v, wantErr %v", err, kt.wantErr)
				return
			}
			if !kt.wantErr && fingerprint == "" {
				t.Errorf("VerifySSHSignature() returned empty fingerprint")
			}
			if !kt.wantErr {
				t.Logf("Verified with fingerprint: %s", fingerprint)
			}
		})
	}
}

func TestVerifySSHSignatureCombinedKeys(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	// Read the combined authorized keys
	authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, "authorized_keys_all"))
	if err != nil {
		t.Fatalf("Failed to read combined authorized keys: %v", err)
	}

	// Test each key type against the combined authorized keys
	keyTypes := []struct {
		name    string
		sigFile string
		wantErr bool
	}{
		{"ed25519", "commit_ed25519_signed.txt", false},
		{"rsa", "commit_rsa_signed.txt", false},
		{"ecdsa_p256", "commit_ecdsa_p256_signed.txt", false},
		{"ecdsa_p384", "commit_ecdsa_p384_signed.txt", false},
		{"ecdsa_p521", "commit_ecdsa_p521_signed.txt", false},
	}

	for _, kt := range keyTypes {
		t.Run(kt.name, func(t *testing.T) {
			// Parse the commit from the fixture file
			commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, kt.sigFile))
			if err != nil {
				t.Fatalf("Failed to parse commit from fixture: %v", err)
			}

			// Build a git.Commit using BuildCommitWithRef
			gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
			if err != nil {
				t.Fatalf("Failed to build commit: %v", err)
			}

			// Verify the signature with combined authorized keys
			fingerprint, err := signatures.VerifySSHSignature(gitCommit.Signature, gitCommit.Encoded, string(authorizedKeys))
			if (err != nil) != kt.wantErr {
				t.Errorf("VerifySSHSignature() error = %v, wantErr %v", err, kt.wantErr)
				return
			}
			if !kt.wantErr && fingerprint == "" {
				t.Errorf("VerifySSHSignature() returned empty fingerprint")
			}
			if !kt.wantErr {
				t.Logf("Verified with fingerprint: %s", fingerprint)
			}
		})
	}
}

func TestBuildCommitWithRefFromFixture(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	tests := []struct {
		name    string
		fixture string
		wantErr bool
		wantSig bool
	}{
		{
			name:    "ed25519 signed commit",
			fixture: "commit_ed25519_signed.txt",
			wantErr: false,
			wantSig: true,
		},
		{
			name:    "rsa signed commit",
			fixture: "commit_rsa_signed.txt",
			wantErr: false,
			wantSig: true,
		},
		{
			name:    "ecdsa p256 signed commit",
			fixture: "commit_ecdsa_p256_signed.txt",
			wantErr: false,
			wantSig: true,
		},
		{
			name:    "ecdsa p384 signed commit",
			fixture: "commit_ecdsa_p384_signed.txt",
			wantErr: false,
			wantSig: true,
		},
		{
			name:    "ecdsa p521 signed commit",
			fixture: "commit_ecdsa_p521_signed.txt",
			wantErr: false,
			wantSig: true,
		},
		{
			name:    "unsigned commit",
			fixture: "commit_unsigned.txt",
			wantErr: false,
			wantSig: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the commit from the fixture file
			commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, tt.fixture))
			if err != nil {
				t.Fatalf("Failed to parse commit from fixture: %v", err)
			}

			// Build a git.Commit using BuildCommitWithRef
			gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildCommitWithRef() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the commit was built correctly
				if gitCommit == nil {
					t.Fatal("BuildCommitWithRef() returned nil commit")
				}

				// Check if signature is present as expected
				hasSig := gitCommit.Signature != ""
				if hasSig != tt.wantSig {
					t.Errorf("BuildCommitWithRef() has signature = %v, want %v", hasSig, tt.wantSig)
				}

				// Verify the encoded data is present
				if len(gitCommit.Encoded) == 0 {
					t.Error("BuildCommitWithRef() returned commit with empty Encoded field")
				}

				// Verify the reference is set correctly
				if gitCommit.Reference != "refs/heads/main" {
					t.Errorf("BuildCommitWithRef() reference = %q, want %q", gitCommit.Reference, "refs/heads/main")
				}

				// Verify the hash is set
				if len(gitCommit.Hash) == 0 {
					t.Error("BuildCommitWithRef() returned commit with empty Hash")
				}

				// Verify author and committer are set
				if gitCommit.Author.Name == "" {
					t.Error("BuildCommitWithRef() returned commit with empty Author.Name")
				}
				if gitCommit.Committer.Name == "" {
					t.Error("BuildCommitWithRef() returned commit with empty Committer.Name")
				}

				// If the commit has a signature, verify it can be extracted
				if tt.wantSig {
					// The signature is stored in gitCommit.Signature, not in gitCommit.Encoded
					// gitCommit.Encoded contains the encoded commit without the signature
					if gitCommit.Signature == "" {
						t.Error("BuildCommitWithRef() returned commit with empty Signature field")
					}
					// Verify the signature contains the expected SSH signature markers
					if !strings.Contains(gitCommit.Signature, "-----BEGIN SSH SIGNATURE-----") {
						t.Error("BuildCommitWithRef() signature does not contain SSH signature start marker")
					}
					if !strings.Contains(gitCommit.Signature, "-----END SSH SIGNATURE-----") {
						t.Error("BuildCommitWithRef() signature does not contain SSH signature end marker")
					}
				}
			}
		})
	}
}

func TestBuildCommitWithRefAndVerifySSH(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	tests := []struct {
		name     string
		fixture  string
		authFile string
		wantErr  bool
	}{
		{
			name:     "ed25519 signed commit",
			fixture:  "commit_ed25519_signed.txt",
			authFile: "authorized_keys_ed25519",
			wantErr:  false,
		},
		{
			name:     "rsa signed commit",
			fixture:  "commit_rsa_signed.txt",
			authFile: "authorized_keys_rsa",
			wantErr:  false,
		},
		{
			name:     "ecdsa p256 signed commit",
			fixture:  "commit_ecdsa_p256_signed.txt",
			authFile: "authorized_keys_ecdsa_p256",
			wantErr:  false,
		},
		{
			name:     "ecdsa p384 signed commit",
			fixture:  "commit_ecdsa_p384_signed.txt",
			authFile: "authorized_keys_ecdsa_p384",
			wantErr:  false,
		},
		{
			name:     "ecdsa p521 signed commit",
			fixture:  "commit_ecdsa_p521_signed.txt",
			authFile: "authorized_keys_ecdsa_p521",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the commit from the fixture file
			commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, tt.fixture))
			if err != nil {
				t.Fatalf("Failed to parse commit from fixture: %v", err)
			}

			// Build a git.Commit using BuildCommitWithRef
			gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, plumbing.ReferenceName("refs/heads/main"))
			if err != nil {
				t.Fatalf("BuildCommitWithRef() error = %v", err)
			}

			// Read the authorized keys
			authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, tt.authFile))
			if err != nil {
				t.Fatalf("Failed to read authorized keys: %v", err)
			}

			// Verify the SSH signature using the git.Commit's VerifySSH method
			fingerprint, err := gitCommit.VerifySSH(string(authorizedKeys))
			if (err != nil) != tt.wantErr {
				t.Errorf("git.Commit.VerifySSH() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if fingerprint == "" {
					t.Error("git.Commit.VerifySSH() returned empty fingerprint")
				}
				t.Logf("Verified with fingerprint: %s", fingerprint)
			}
		})
	}
}

func TestBuildCommitWithRefWithDifferentRefs(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	// Parse a signed commit from the fixture file
	commitObj, err := testutils.ParseCommitFromFixture(filepath.Join(testDataDir, "commit_ed25519_signed.txt"))
	if err != nil {
		t.Fatalf("Failed to parse commit from fixture: %v", err)
	}

	tests := []struct {
		name    string
		ref     plumbing.ReferenceName
		wantRef string
	}{
		{
			name:    "branch reference",
			ref:     plumbing.ReferenceName("refs/heads/main"),
			wantRef: "refs/heads/main",
		},
		{
			name:    "tag reference",
			ref:     plumbing.ReferenceName("refs/tags/v1.0.0"),
			wantRef: "refs/tags/v1.0.0",
		},
		{
			name:    "remote branch reference",
			ref:     plumbing.ReferenceName("refs/remotes/origin/main"),
			wantRef: "refs/remotes/origin/main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a git.Commit using BuildCommitWithRef with different references
			gitCommit, err := gogit.BuildCommitWithRef(commitObj, nil, tt.ref)
			if err != nil {
				t.Fatalf("BuildCommitWithRef() error = %v", err)
			}

			// Verify the reference is set correctly
			if gitCommit.Reference != tt.wantRef {
				t.Errorf("BuildCommitWithRef() reference = %q, want %q", gitCommit.Reference, tt.wantRef)
			}

			// Verify other fields are still set correctly
			if len(gitCommit.Hash) == 0 {
				t.Error("BuildCommitWithRef() returned commit with empty Hash")
			}
			if gitCommit.Signature == "" {
				t.Error("BuildCommitWithRef() returned commit with empty Signature")
			}
		})
	}
}

func TestBuildTagFromFixture(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	tests := []struct {
		name    string
		fixture string
		wantErr bool
		wantSig bool
	}{
		{
			name:    "ed25519 signed tag",
			fixture: "tag_ed25519_signed.txt",
			wantErr: false,
			wantSig: true,
		},
		{
			name:    "rsa signed tag",
			fixture: "tag_rsa_signed.txt",
			wantErr: false,
			wantSig: true,
		},
		{
			name:    "ecdsa p256 signed tag",
			fixture: "tag_ecdsa_p256_signed.txt",
			wantErr: false,
			wantSig: true,
		},
		{
			name:    "ecdsa p384 signed tag",
			fixture: "tag_ecdsa_p384_signed.txt",
			wantErr: false,
			wantSig: true,
		},
		{
			name:    "ecdsa p521 signed tag",
			fixture: "tag_ecdsa_p521_signed.txt",
			wantErr: false,
			wantSig: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the tag from the fixture file
			tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, tt.fixture))
			if err != nil {
				t.Fatalf("Failed to parse tag from fixture: %v", err)
			}

			// Build a git.Tag using BuildTag
			gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildTag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				// Verify the tag was built correctly
				if gitTag == nil {
					t.Fatal("BuildTag() returned nil tag")
				}

				// Check if signature is present as expected
				hasSig := gitTag.Signature != ""
				if hasSig != tt.wantSig {
					t.Errorf("BuildTag() has signature = %v, want %v", hasSig, tt.wantSig)
				}

				// Verify the encoded data is present
				if len(gitTag.Encoded) == 0 {
					t.Error("BuildTag() returned tag with empty Encoded field")
				}

				// Verify the name is set correctly
				if gitTag.Name == "" {
					t.Error("BuildTag() returned tag with empty Name")
				}

				// Verify the hash is set
				if len(gitTag.Hash) == 0 {
					t.Error("BuildTag() returned tag with empty Hash")
				}

				// Verify author is set
				if gitTag.Author.Name == "" {
					t.Error("BuildTag() returned tag with empty Author.Name")
				}

				// If the tag has a signature, verify it can be extracted
				if tt.wantSig {
					if gitTag.Signature == "" {
						t.Error("BuildTag() returned tag with empty Signature field")
					}
					// Verify the signature contains the expected SSH signature markers
					if !strings.Contains(gitTag.Signature, "-----BEGIN SSH SIGNATURE-----") {
						t.Error("BuildTag() signature does not contain SSH signature start marker")
					}
					if !strings.Contains(gitTag.Signature, "-----END SSH SIGNATURE-----") {
						t.Error("BuildTag() signature does not contain SSH signature end marker")
					}
				}
			}
		})
	}
}

func TestVerifySSHSignatureForTags(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	// Test cases for each key type using fixtures
	keyTypes := []struct {
		name     string
		sigFile  string
		authFile string
		wantErr  bool
	}{
		{"ed25519 valid signature", "tag_ed25519_signed.txt", "authorized_keys_ed25519", false},
		{"rsa valid signature", "tag_rsa_signed.txt", "authorized_keys_rsa", false},
		{"ecdsa_p256 valid signature", "tag_ecdsa_p256_signed.txt", "authorized_keys_ecdsa_p256", false},
		{"ecdsa_p384 valid signature", "tag_ecdsa_p384_signed.txt", "authorized_keys_ecdsa_p384", false},
		{"ecdsa_p521 valid signature", "tag_ecdsa_p521_signed.txt", "authorized_keys_ecdsa_p521", false},
	}

	for _, kt := range keyTypes {
		t.Run(kt.name, func(t *testing.T) {
			// Parse the tag from the fixture file
			tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, kt.sigFile))
			if err != nil {
				t.Fatalf("Failed to parse tag from fixture: %v", err)
			}

			// Build a git.Tag using BuildTag
			gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
			if err != nil {
				t.Fatalf("Failed to build tag: %v", err)
			}

			// Read the authorized keys
			authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, kt.authFile))
			if err != nil {
				t.Fatalf("Failed to read authorized keys: %v", err)
			}

			// Verify the signature using the git.Tag's Signature and Encoded fields
			fingerprint, err := signatures.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, string(authorizedKeys))
			if (err != nil) != kt.wantErr {
				t.Errorf("VerifySSHSignature() error = %v, wantErr %v", err, kt.wantErr)
				return
			}
			if !kt.wantErr && fingerprint == "" {
				t.Errorf("VerifySSHSignature() returned empty fingerprint")
			}
			if !kt.wantErr {
				t.Logf("Verified with fingerprint: %s", fingerprint)
			}
		})
	}

	// Test error cases
	t.Run("empty signature", func(t *testing.T) {
		authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, "authorized_keys_ed25519"))
		if err != nil {
			t.Fatalf("Failed to read authorized keys: %v", err)
		}

		// Parse the tag from the fixture file
		tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, "tag_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse tag from fixture: %v", err)
		}

		// Build a git.Tag using BuildTag
		gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
		if err != nil {
			t.Fatalf("Failed to build tag: %v", err)
		}

		fingerprint, err := signatures.VerifySSHSignature("", gitTag.Encoded, string(authorizedKeys))
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
		authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, "authorized_keys_ed25519"))
		if err != nil {
			t.Fatalf("Failed to read authorized keys: %v", err)
		}

		// Parse the tag from the fixture file
		tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, "tag_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse tag from fixture: %v", err)
		}

		// Build a git.Tag using BuildTag
		gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
		if err != nil {
			t.Fatalf("Failed to build tag: %v", err)
		}

		fingerprint, err := signatures.VerifySSHSignature(gitTag.Signature, []byte{}, string(authorizedKeys))
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
		// Parse the tag from the fixture file
		tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, "tag_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse tag from fixture: %v", err)
		}

		// Build a git.Tag using BuildTag
		gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
		if err != nil {
			t.Fatalf("Failed to build tag: %v", err)
		}

		// Use a different key that won't match
		wrongKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEyM97VxLgOCuB9Eg5cDtTc8ogkdM1xAyJhzODB9cK1 wrong@example.com"

		fingerprint, err := signatures.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, wrongKey)
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

	t.Run("invalid signature", func(t *testing.T) {
		authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, "authorized_keys_ed25519"))
		if err != nil {
			t.Fatalf("Failed to read authorized keys: %v", err)
		}

		// Parse the tag from the fixture file
		tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, "tag_ed25519_signed.txt"))
		if err != nil {
			t.Fatalf("Failed to parse tag from fixture: %v", err)
		}

		// Build a git.Tag using BuildTag
		gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
		if err != nil {
			t.Fatalf("Failed to build tag: %v", err)
		}

		invalidSig := "-----BEGIN SSH SIGNATURE-----\n invalid\n -----END SSH SIGNATURE-----"

		fingerprint, err := signatures.VerifySSHSignature(invalidSig, gitTag.Encoded, string(authorizedKeys))
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
}

func TestVerifySSHSignatureForTagsAllKeyTypes(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	// Test cases for each key type
	keyTypes := []struct {
		name     string
		sigFile  string
		authFile string
		wantErr  bool
	}{
		{"ed25519", "tag_ed25519_signed.txt", "authorized_keys_ed25519", false},
		{"rsa", "tag_rsa_signed.txt", "authorized_keys_rsa", false},
		{"ecdsa_p256", "tag_ecdsa_p256_signed.txt", "authorized_keys_ecdsa_p256", false},
		{"ecdsa_p384", "tag_ecdsa_p384_signed.txt", "authorized_keys_ecdsa_p384", false},
		{"ecdsa_p521", "tag_ecdsa_p521_signed.txt", "authorized_keys_ecdsa_p521", false},
	}

	for _, kt := range keyTypes {
		t.Run(kt.name, func(t *testing.T) {
			// Parse the tag from the fixture file
			tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, kt.sigFile))
			if err != nil {
				t.Fatalf("Failed to parse tag from fixture: %v", err)
			}

			// Build a git.Tag using BuildTag
			gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
			if err != nil {
				t.Fatalf("Failed to build tag: %v", err)
			}

			// Read the authorized keys
			authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, kt.authFile))
			if err != nil {
				t.Fatalf("Failed to read authorized keys: %v", err)
			}

			// Verify the signature using the git.Tag's Signature and Encoded fields
			fingerprint, err := signatures.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, string(authorizedKeys))
			if (err != nil) != kt.wantErr {
				t.Errorf("VerifySSHSignature() error = %v, wantErr %v", err, kt.wantErr)
				return
			}
			if !kt.wantErr && fingerprint == "" {
				t.Errorf("VerifySSHSignature() returned empty fingerprint")
			}
			if !kt.wantErr {
				t.Logf("Verified with fingerprint: %s", fingerprint)
			}
		})
	}
}

func TestVerifySSHSignatureForTagsCombinedKeys(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	// Read the combined authorized keys
	authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, "authorized_keys_all"))
	if err != nil {
		t.Fatalf("Failed to read combined authorized keys: %v", err)
	}

	// Test each key type against the combined authorized keys
	keyTypes := []struct {
		name    string
		sigFile string
		wantErr bool
	}{
		{"ed25519", "tag_ed25519_signed.txt", false},
		{"rsa", "tag_rsa_signed.txt", false},
		{"ecdsa_p256", "tag_ecdsa_p256_signed.txt", false},
		{"ecdsa_p384", "tag_ecdsa_p384_signed.txt", false},
		{"ecdsa_p521", "tag_ecdsa_p521_signed.txt", false},
	}

	for _, kt := range keyTypes {
		t.Run(kt.name, func(t *testing.T) {
			// Parse the tag from the fixture file
			tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, kt.sigFile))
			if err != nil {
				t.Fatalf("Failed to parse tag from fixture: %v", err)
			}

			// Build a git.Tag using BuildTag
			gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
			if err != nil {
				t.Fatalf("Failed to build tag: %v", err)
			}

			// Verify the signature with combined authorized keys
			fingerprint, err := signatures.VerifySSHSignature(gitTag.Signature, gitTag.Encoded, string(authorizedKeys))
			if (err != nil) != kt.wantErr {
				t.Errorf("VerifySSHSignature() error = %v, wantErr %v", err, kt.wantErr)
				return
			}
			if !kt.wantErr && fingerprint == "" {
				t.Errorf("VerifySSHSignature() returned empty fingerprint")
			}
			if !kt.wantErr {
				t.Logf("Verified with fingerprint: %s", fingerprint)
			}
		})
	}
}

func TestBuildTagAndVerifySSH(t *testing.T) {
	testDataDir := filepath.Join("testdata", "ssh_signatures")

	tests := []struct {
		name     string
		fixture  string
		authFile string
		wantErr  bool
	}{
		{
			name:     "ed25519 signed tag",
			fixture:  "tag_ed25519_signed.txt",
			authFile: "authorized_keys_ed25519",
			wantErr:  false,
		},
		{
			name:     "rsa signed tag",
			fixture:  "tag_rsa_signed.txt",
			authFile: "authorized_keys_rsa",
			wantErr:  false,
		},
		{
			name:     "ecdsa p256 signed tag",
			fixture:  "tag_ecdsa_p256_signed.txt",
			authFile: "authorized_keys_ecdsa_p256",
			wantErr:  false,
		},
		{
			name:     "ecdsa p384 signed tag",
			fixture:  "tag_ecdsa_p384_signed.txt",
			authFile: "authorized_keys_ecdsa_p384",
			wantErr:  false,
		},
		{
			name:     "ecdsa p521 signed tag",
			fixture:  "tag_ecdsa_p521_signed.txt",
			authFile: "authorized_keys_ecdsa_p521",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the tag from the fixture file
			tagObj, err := testutils.ParseTagFromFixture(filepath.Join(testDataDir, tt.fixture))
			if err != nil {
				t.Fatalf("Failed to parse tag from fixture: %v", err)
			}

			// Build a git.Tag using BuildTag
			gitTag, err := gogit.BuildTag(tagObj, plumbing.ReferenceName("refs/tags/test-tag"))
			if err != nil {
				t.Fatalf("BuildTag() error = %v", err)
			}

			// Read the authorized keys
			authorizedKeys, err := os.ReadFile(filepath.Join(testDataDir, tt.authFile))
			if err != nil {
				t.Fatalf("Failed to read authorized keys: %v", err)
			}

			// Verify the SSH signature using the git.Tag's VerifySSH method
			fingerprint, err := gitTag.VerifySSH(string(authorizedKeys))
			if (err != nil) != tt.wantErr {
				t.Errorf("git.Tag.VerifySSH() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if fingerprint == "" {
					t.Error("git.Tag.VerifySSH() returned empty fingerprint")
				}
				t.Logf("Verified with fingerprint: %s", fingerprint)
			}
		})
	}
}
