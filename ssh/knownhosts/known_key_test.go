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
	"encoding/base64"
	"testing"

	. "github.com/onsi/gomega"
)

// knownHostsFixture is known_hosts fixture in the expected
// format.
var knownHostsFixture = `github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==`

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

func Test_parseKnownHosts_matches(t *testing.T) {
	tests := []struct {
		name        string
		fingerprint []byte
		wantMatches bool
	}{
		{
			name:        "good sha256 hostkey",
			fingerprint: sha256Fingerprint("nThbg6kXUpJWGl7E1IGOCspRomTxdCARLviKw6E5SY8"),
			wantMatches: true,
		},
		{
			name:        "bad sha256 hostkey",
			fingerprint: sha256Fingerprint("ROQFvPThGrW4RuWLoL9tq9I9zJ42fK4XywyRtbOz/EQ"),
			wantMatches: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			knownKeys, err := ParseKnownHosts(knownHostsFixture)
			if err != nil {
				t.Error(err)
				return
			}
			matches := knownKeys[0].Matches("github.com", tt.fingerprint)
			g.Expect(matches).To(Equal(tt.wantMatches))
		})
	}
}

func Test_parseKnownHosts(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		wantErr bool
	}{
		{
			name:    "empty file",
			fixture: "",
			wantErr: false,
		},
		{
			name:    "single host",
			fixture: `github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==`,
			wantErr: false,
		},
		{
			name: "single host with comment",
			fixture: `# github.com
github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==`,
			wantErr: false,
		},
		{
			name: "multiple hosts with comments",
			fixture: `# github.com
github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==
# gitlab.com
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf`,
		},
		{
			name: "no host key, only comments",
			fixture: `# example.com
#github.com
# gitlab.com`,
			wantErr: false,
		},
		{
			name:    "invalid host entry",
			fixture: `github.com ssh-rsa`,
			wantErr: true,
		},
		{
			name:    "invalid content",
			fixture: `some random text`,
			wantErr: true,
		},
		{
			name: "invalid line with valid host key",
			fixture: `some random text
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf`,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			_, err := ParseKnownHosts(tt.fixture)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func sha256Fingerprint(in string) []byte {
	d, err := base64.RawStdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}
	var out [32]byte
	copy(out[:], d)
	return out[:]
}
