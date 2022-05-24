/*
Copyright 2022 The Flux authors

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

package knownhosts

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_matchHashedHost(t *testing.T) {
	tests := []struct {
		name       string
		hashedHost string
		host       string
		match      bool
		wantErr    string
	}{
		{
			name:       "match valid known host",
			hashedHost: "|1|vApZG0Ybr4rHfTb69+cjjFIGIv0=|M5sSXen14encOvQAy0gseRahnJw=",
			host:       "[127.0.0.1]:44167",
			match:      true,
		},
		{
			name:    "empty known host errors",
			wantErr: "hashed known host must begin with '|'",
		},
		{
			name:       "unhashed known host errors",
			hashedHost: "[127.0.0.1]:44167",
			wantErr:    "hashed known host must begin with '|'",
		},
		{
			name:       "invalid known host format errors",
			hashedHost: "|1M5sSXen14encOvQAy0gseRahnJw=",
			wantErr:    "invalid format for hashed known host",
		},
		{
			name:       "invalid hash type errors",
			hashedHost: "|2|vApZG0Ybr4rHfTb69+cjjFIGIv0=|M5sSXen14encOvQAy0gseRahnJw=",
			wantErr:    "unsupported hash type",
		},
		{
			name:       "invalid base64 component[2] errors",
			hashedHost: "|1|azz|M5sSXen14encOvQAy0gseRahnJw=",
			wantErr:    "cannot decode hashed known host",
		},
		{
			name:       "invalid base64 component[3] errors",
			hashedHost: "|1|M5sSXen14encOvQAy0gseRahnJw=|azz",
			wantErr:    "cannot decode hashed known host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			matched, err := matchHashedHost(tt.hashedHost, tt.host)

			if tt.wantErr == "" {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(matched).To(Equal(tt.match))
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestParseKnownHosts(t *testing.T) {
	known_host := "11.101.41.142 ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBLBfOI4ma6GtSaWssT8pqJ7kVxuMfcYhTIs5p0TiiY7Wz8WVArUzzQjoKUJ60HT5CqHmOMb8ux6nDIXNRamf+VE="

	kk, err := ParseKnownHosts(known_host)
	g := NewWithT(t)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(kk)).To(Equal(1))
	known_host = known_host + "invalidbase"

	kk, err = ParseKnownHosts(known_host)
	g.Expect(err).To(HaveOccurred())
	g.Expect(len(kk)).To(Equal(0))
}
