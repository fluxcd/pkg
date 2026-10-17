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

package v1

import "testing"

func TestParseSPIFFEID(t *testing.T) {
	tests := []struct {
		name            string
		value           string
		wantID          string
		wantPrefix      string
		wantTrustDomain string
		wantSelf        bool
		wantError       bool
	}{
		{
			name:   "exact SPIFFE ID",
			value:  "spiffe://example.org/foo/bar",
			wantID: "spiffe://example.org/foo/bar",
		},
		{
			name:       "SPIFFE ID prefix",
			value:      "spiffe://example.org/foo/",
			wantPrefix: "spiffe://example.org/foo",
		},
		{
			name:            "SPIFFE trust domain",
			value:           "spiffe://example.org",
			wantTrustDomain: "example.org",
		},
		{
			name:     "self trust domain",
			value:    "spiffe://self",
			wantSelf: true,
		},
		{name: "invalid SPIFFE ID", value: "nope", wantError: true},
		{name: "invalid SPIFFE ID prefix", value: "spiffe://example.org/a b/", wantError: true},
		{name: "prefix without path", value: "spiffe://example.org/", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSPIFFEID(tt.value)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected an error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			switch {
			case tt.wantID != "":
				if got.ID == nil || got.ID.String() != tt.wantID {
					t.Fatalf("got ID %v, want %q", got.ID, tt.wantID)
				}
			case tt.wantPrefix != "":
				if got.Prefix == nil || got.Prefix.String() != tt.wantPrefix {
					t.Fatalf("got Prefix %v, want %q", got.Prefix, tt.wantPrefix)
				}
			case tt.wantTrustDomain != "":
				if got.TrustDomain == nil || got.TrustDomain.String() != tt.wantTrustDomain {
					t.Fatalf("got TrustDomain %v, want %q", got.TrustDomain, tt.wantTrustDomain)
				}
			case tt.wantSelf:
				if !got.UseSelfTrustDomain {
					t.Fatalf("got %+v, want UseSelfTrustDomain", got)
				}
			}

			// Exactly one field must be set.
			set := 0
			if got.ID != nil {
				set++
			}
			if got.Prefix != nil {
				set++
			}
			if got.TrustDomain != nil {
				set++
			}
			if got.UseSelfTrustDomain {
				set++
			}
			if set != 1 {
				t.Fatalf("got %d fields set, want exactly 1: %+v", set, got)
			}
		})
	}
}
