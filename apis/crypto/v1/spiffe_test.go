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

import (
	"testing"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
)

func TestParseSPIFFEID(t *testing.T) {
	tests := []struct {
		name      string
		value     string
		want      *SPIFFEPeerAuthorization
		wantError bool
	}{
		{
			name:  "self trust domain",
			value: "spiffe://" + SPIFFETrustDomainSelf,
			want:  &SPIFFEPeerAuthorization{UseSelfTrustDomain: true},
		},
		{
			name:  "exact ID",
			value: "spiffe://example.org/foo",
			want:  &SPIFFEPeerAuthorization{ID: mustSPIFFEID(t, "spiffe://example.org/foo")},
		},
		{
			name:  "prefix",
			value: "spiffe://example.org/foo/",
			want:  &SPIFFEPeerAuthorization{Prefix: mustSPIFFEID(t, "spiffe://example.org/foo")},
		},
		{
			name:  "trust domain",
			value: "spiffe://example.org",
			want:  &SPIFFEPeerAuthorization{TrustDomain: mustTrustDomain(t, "example.org")},
		},
		{name: "invalid ID", value: "not-a-spiffe-id", wantError: true},
		{name: "invalid prefix", value: "spiffe:///", wantError: true},
		{name: "prefix without path", value: "spiffe://example.org/", wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSPIFFEID(tt.value)
			if (err != nil) != tt.wantError {
				t.Fatalf("got error %v, want error=%v", err, tt.wantError)
			}
			if tt.wantError {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			assertPeerAuthorization(t, got, tt.want)
		})
	}
}

func TestMTLSSPIFFEServerIDExtract(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		got, err := AnnotationMTLSSPIFFEServerID.Extract(map[string]string{})
		if err != nil || got != nil {
			t.Fatalf("got (%+v, %v), want (nil, nil)", got, err)
		}
	})
	t.Run("present", func(t *testing.T) {
		got, err := AnnotationMTLSSPIFFEServerID.Extract(map[string]string{
			AnnotationMTLSSPIFFEServerID.String(): "spiffe://example.org/foo",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		assertPeerAuthorization(t, got, &SPIFFEPeerAuthorization{
			ID: mustSPIFFEID(t, "spiffe://example.org/foo"),
		})
	})
	t.Run("invalid", func(t *testing.T) {
		got, err := AnnotationMTLSSPIFFEServerID.Extract(map[string]string{
			AnnotationMTLSSPIFFEServerID.String(): "nope",
		})
		if err == nil || got != nil {
			t.Fatalf("got (%+v, %v), want (nil, error)", got, err)
		}
	})
}

func mustSPIFFEID(t *testing.T, value string) *spiffeid.ID {
	t.Helper()
	id, err := spiffeid.FromString(value)
	if err != nil {
		t.Fatalf("parsing SPIFFE ID %q: %v", value, err)
	}
	return &id
}

func mustTrustDomain(t *testing.T, value string) *spiffeid.TrustDomain {
	t.Helper()
	trustDomain, err := spiffeid.TrustDomainFromString(value)
	if err != nil {
		t.Fatalf("parsing SPIFFE trust domain %q: %v", value, err)
	}
	return &trustDomain
}

func assertPeerAuthorization(t *testing.T, got, want *SPIFFEPeerAuthorization) {
	t.Helper()
	if got == nil {
		t.Fatalf("got nil, want %+v", want)
	}
	switch {
	case got.ID != nil && want.ID != nil:
		if got.ID.String() != want.ID.String() {
			t.Fatalf("ID = %q, want %q", got.ID, want.ID)
		}
	case got.ID != nil || want.ID != nil:
		t.Fatalf("ID = %v, want %v", got.ID, want.ID)
	}
	switch {
	case got.Prefix != nil && want.Prefix != nil:
		if got.Prefix.String() != want.Prefix.String() {
			t.Fatalf("Prefix = %q, want %q", got.Prefix, want.Prefix)
		}
	case got.Prefix != nil || want.Prefix != nil:
		t.Fatalf("Prefix = %v, want %v", got.Prefix, want.Prefix)
	}
	switch {
	case got.TrustDomain != nil && want.TrustDomain != nil:
		if got.TrustDomain.String() != want.TrustDomain.String() {
			t.Fatalf("TrustDomain = %q, want %q", got.TrustDomain, want.TrustDomain)
		}
	case got.TrustDomain != nil || want.TrustDomain != nil:
		t.Fatalf("TrustDomain = %v, want %v", got.TrustDomain, want.TrustDomain)
	}
	if got.UseSelfTrustDomain != want.UseSelfTrustDomain {
		t.Fatalf("UseSelfTrustDomain = %v, want %v", got.UseSelfTrustDomain, want.UseSelfTrustDomain)
	}
}
