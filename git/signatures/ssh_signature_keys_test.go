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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// these tests are in the same package to test private getPublicKeyFingerprint function

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
			keys, err := ParseAuthorizedKeys(tt.authorizedKeys)
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
					fingerprint := getPublicKeyFingerprint(key)
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

			keys, err := ParseAuthorizedKeys(string(authorizedKeys))
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
				fingerprint := getPublicKeyFingerprint(key)
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

			keys, err := ParseAuthorizedKeys(combinedKeys.String())
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseAuthorizedKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(keys) != tt.wantCount {
				t.Errorf("ParseAuthorizedKeys() got %d keys, want %d", len(keys), tt.wantCount)
			}

			// Verify that each key has a valid fingerprint
			for i, key := range keys {
				fingerprint := getPublicKeyFingerprint(key)
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
