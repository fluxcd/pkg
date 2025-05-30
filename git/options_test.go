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
	"net/url"
	"testing"

	. "github.com/onsi/gomega"
)

const (
	// privateKeyFixture is a randomly generated password less
	// 512bit RSA private key.
	privateKeyFixture = `-----BEGIN RSA PRIVATE KEY-----
MIICXAIBAAKBgQCrakELAKxozvwJijQEggYlTvS1QTZx1DaBwOhW/4kRSuR21plu
xuQeyuUiztoWeb9jgW7wjzG4j1PIJjdbsgjPIcIZ4PBY7JeEW+QRopfwuN8MHXNp
uTLgIHbkmhoOg5qBEcjzO/lEOOPpV0EmbObgqv3+wRmLJrgfzWl/cTtRewIDAQAB
AoGAawKFImpEN5Xn78iwWpQVZBsbV0AjzgHuGSiloxIZrorzf2DPHkHZzYNaclVx
/o/4tBTsfg7WumH3qr541qyZJDgU7iRMABwmx0v1vm2wQiX7NJzLzH2E9vlMC3mw
d8S99g9EqRuNH98XX8su34B9WGRPqiKvEm0RW8Hideo2/KkCQQDbs6rHcriKQyPB
paidHZAfguu0eVbyHT2EgLgRboWE+tEAqFEW2ycqNL3VPz9fRvwexbB6rpOcPpQJ
DEL4XB2XAkEAx7xJz8YlCQ2H38xggK8R8EUXF9Zhb0fqMJHMNmao1HCHVMtbsa8I
jR2EGyQ4CaIqNG5tdWukXQSJrPYDRWNvvQJAZX3rP7XUYDLB2twvN12HzbbKMhX3
v2MYnxRjc9INpi/Dyzz2MMvOnOW+aDuOh/If2AtVCmeJUx1pf4CFk3viQwJBAKyC
t824+evjv+NQBlme3AOF6PgxtV4D4wWoJ5Uk/dTejER0j/Hbl6sqPxuiILRRV9qJ
Ngkgu4mLjc3RfenEhJECQAx8zjWUE6kHHPGAd9DfiAIQ4bChqnyS0Nwb9+Gd4hSE
P0Ah10mHiK/M0o3T8Eanwum0gbQHPnOwqZgsPkwXRqQ=
-----END RSA PRIVATE KEY-----`

	// privateKeyPassphraseFixture is a randomly generated
	// 512bit RSA private key with password foobar.
	privateKeyPassphraseFixture = `-----BEGIN RSA PRIVATE KEY-----
Proc-Type: 4,ENCRYPTED
DEK-Info: AES-256-CBC,0B016973B2A761D31E6B388D0F327C35

X9GET/qAyZkAJBl/RK+1XX75NxONgdUfZDw7PIYi/g+Efh3Z5zH5kh/dx9lxH5ZG
HGCqPAeMO/ofGDGtDULWW6iqDUFRu5gPgEVSCnnbqoHNU325WHhXdhejVAItwObC
IpL/zYfs2+gDHXct/n9FJ/9D/EGXZihwPqYaK8GQSfZAxz0QjLuh0wU1qpbm3y3N
q+o9FLv3b2Ys/tCJOUsYVQOYLSrZEI77y1ii3nWgQ8lXiTJbBUKzuq4f1YWeO8Ah
RZbdhTa57AF5lUaRtL7Nrm3HJUrK1alBbU7HHyjeW4Q4n/D3fiRDC1Mh2Bi4EOOn
wGctSx4kHsZGhJv5qwKqqPEFPhUzph8D2tm2TABk8HJa5KJFDbGrcfvk2uODAoZr
MbcpIxCfl8oB09bWfY6tDQjyvwSYYo2Phdwm7kT92xc=
-----END RSA PRIVATE KEY-----`

	// knownHostsFixture is known_hosts fixture in the expected
	// format.
	knownHostsFixture = `github.com ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEAq2A7hRGmdnm9tUDbO9IDSwBK6TbQa+PXYPCPy6rbTrTtw7PHkccKrpp0yVhp5HdEIcKr6pLlVDBfOLX9QUsyCOV0wzfjIJNlGEYsdlLJizHhbn2mUjvSAHQqZETYP81eFzLQNnPHt4EVVUh7VfDESU84KezmD5QlWpXLmvU31/yMf+Se8xhHTvKSCZIFImWwoG6mbUoWf9nzpIoaSjB+weqqUUmpaaasXVal72J+UX2B+2RPW3RcT0eOzQgqlJL3RKrTJvdsjE3JEAvGq3lGHSZXy28G3skua2SmVi/w4yCE6gbODqnTWlg7+wC604ydGXA8VJiS5ap43JXiUFFAaQ==`
)

func TestAuthOptions_Validate(t *testing.T) {
	tests := []struct {
		name    string
		opts    AuthOptions
		wantErr string
	}{
		{
			name: "HTTP transport with password requires user",
			opts: AuthOptions{
				Transport: HTTP,
				Password:  "foo",
			},
			wantErr: "invalid 'http' auth option: 'password' requires 'username' to be set",
		},
		{
			name: "Valid HTTP transport",
			opts: AuthOptions{
				Transport: HTTP,
				Username:  "example",
				Password:  "foo",
			},
		},
		{
			name: "HTTPS transport with password requires user",
			opts: AuthOptions{
				Transport: HTTPS,
				Password:  "foo",
			},
			wantErr: "invalid 'https' auth option: 'password' requires 'username' to be set",
		},
		{
			name: "Valid HTTPS transport",
			opts: AuthOptions{
				Transport: HTTPS,
				Username:  "example",
				Password:  "foo",
			},
		},
		{
			name: "Valid HTTPS without any config",
			opts: AuthOptions{
				Transport: HTTPS,
			},
		},
		{
			name: "SSH transport requires host",
			opts: AuthOptions{
				Transport: SSH,
			},
			wantErr: "invalid 'ssh' auth option: 'host' is required",
		},
		{
			name: "SSH transport requires identity",
			opts: AuthOptions{
				Transport: SSH,
				Host:      "github.com:22",
			},
			wantErr: "invalid 'ssh' auth option: 'identity' is required",
		},
		{
			name: "SSH transport requires known_hosts",
			opts: AuthOptions{
				Transport: SSH,
				Host:      "github.com:22",
				Identity:  []byte(privateKeyFixture),
			},
			wantErr: "invalid 'ssh' auth option: 'known_hosts' is required",
		},
		{
			name:    "Requires transport",
			opts:    AuthOptions{},
			wantErr: "no transport type set",
		},
		{
			name: "Valid SSH transport",
			opts: AuthOptions{
				Host:       "github.com:22",
				Transport:  SSH,
				Identity:   []byte(privateKeyPassphraseFixture),
				Password:   "foobar",
				KnownHosts: []byte(knownHostsFixture),
			},
		},
		{
			name:    "No transport",
			opts:    AuthOptions{},
			wantErr: "no transport type set",
		},
		{
			name: "Unknown transport",
			opts: AuthOptions{
				Transport: "foo",
			},
			wantErr: "unknown transport 'foo'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			got := tt.opts.Validate()
			if tt.wantErr != "" {
				g.Expect(got.Error()).To(ContainSubstring(tt.wantErr))
				return
			}
			g.Expect(got).ToNot(HaveOccurred())
		})
	}
}

func TestAuthOptionsFromData(t *testing.T) {
	tests := []struct {
		name     string
		URL      string
		data     map[string][]byte
		wantFunc func(g *WithT, opts *AuthOptions)
		wantErr  string
	}{
		{
			name: "Sets only relevant values from Secret for HTTPS with basic auth",
			URL:  "https://example.com",
			data: map[string][]byte{
				dataKeyUsername:   []byte("example"),
				dataKeyPassword:   []byte("secret"),
				dataKeyIdentity:   []byte(privateKeyFixture),
				dataKeyKnownHosts: []byte(knownHostsFixture),
				dataKeyCACert:     []byte("mock"),
			},

			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.Username).To(Equal("example"))
				g.Expect(opts.Password).To(Equal("secret"))
				g.Expect(opts.BearerToken).To(Equal(""))
				g.Expect(opts.Identity).To(BeNil())
				g.Expect(opts.KnownHosts).To(BeNil())
				g.Expect(opts.CAFile).To(BeEquivalentTo("mock"))
			},
		},
		{
			name: "Sets only relevant values from Secret for HTTPS with bearer token",
			URL:  "https://example.com",
			data: map[string][]byte{
				dataKeyUsername:    []byte("example"),
				dataKeyPassword:    []byte("secret"),
				dataKeyBearerToken: []byte("token"),
				dataKeyIdentity:    []byte(privateKeyFixture),
				dataKeyKnownHosts:  []byte(knownHostsFixture),
				dataKeyTLSCert:     []byte("mock"),
				dataKeyTLSKey:      []byte("mock"),
				dataKeyCACert:      []byte("mock"),
			},

			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.Username).To(Equal(""))
				g.Expect(opts.Password).To(Equal(""))
				g.Expect(opts.BearerToken).To(Equal("token")) // Preferred over basic auth when provided
				g.Expect(opts.Identity).To(BeNil())
				g.Expect(opts.KnownHosts).To(BeNil())
				g.Expect(opts.ClientCert).To(BeEquivalentTo("mock"))
				g.Expect(opts.ClientKey).To(BeEquivalentTo("mock"))
				g.Expect(opts.CAFile).To(BeEquivalentTo("mock"))
			},
		},
		{
			name: "Sets only relevant values from Secret for SSH",
			URL:  "ssh://example.com",
			data: map[string][]byte{
				dataKeyUsername:    []byte("example"),
				dataKeyPassword:    []byte("secret"),
				dataKeyBearerToken: []byte("token"),
				dataKeyIdentity:    []byte(privateKeyFixture),
				dataKeyKnownHosts:  []byte(knownHostsFixture),
				dataKeyTLSCert:     []byte("mock"),
				dataKeyTLSKey:      []byte("mock"),
				dataKeyCACert:      []byte("mock"),
			},

			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.Username).To(Equal(DefaultPublicKeyAuthUser)) // Not the specified username
				g.Expect(opts.Password).To(Equal("secret"))                 // For passphrase protected ssh key
				g.Expect(opts.BearerToken).To(Equal(""))
				g.Expect(opts.Identity).To(BeEquivalentTo(privateKeyFixture))
				g.Expect(opts.KnownHosts).To(BeEquivalentTo(knownHostsFixture))
				g.Expect(opts.ClientCert).To(BeNil())
				g.Expect(opts.ClientKey).To(BeNil())
				g.Expect(opts.CAFile).To(BeNil())
			},
		},
		{
			name: "Sets default user for SSH",
			URL:  "ssh://example.com",
			data: map[string][]byte{
				dataKeyIdentity:   []byte(privateKeyFixture),
				dataKeyKnownHosts: []byte(knownHostsFixture),
			},
			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.Username).To(Equal(DefaultPublicKeyAuthUser))
			},
		},
		{
			name: "Sets user for SSH from URL",
			URL:  "ssh://user@example.com",
			data: map[string][]byte{
				dataKeyIdentity:   []byte(privateKeyFixture),
				dataKeyKnownHosts: []byte(knownHostsFixture),
			},
			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.Username).To(Equal("user"))
			},
		},
		{
			name: "Sets client cert for HTTPS using tls.crt",
			URL:  "https://example.com",
			data: map[string][]byte{
				dataKeyTLSCert: []byte("mock"),
			},
			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.ClientCert).To(BeEquivalentTo("mock"))
			},
		},
		{
			name: "Sets client key for HTTPS using tls.key",
			URL:  "https://example.com",
			data: map[string][]byte{
				dataKeyTLSKey: []byte("mock"),
			},
			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.ClientKey).To(BeEquivalentTo("mock"))
			},
		},
		{
			name: "Sets CAFile for HTTPS using ca.crt",
			URL:  "https://example.com",
			data: map[string][]byte{
				dataKeyCACert: []byte("mock"),
			},
			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.CAFile).To(BeEquivalentTo("mock"))
			},
		},
		{
			name: "Sets CAFile for HTTPS using caFile",
			URL:  "https://example.com",
			data: map[string][]byte{
				dataKeyCAFile: []byte("mock"),
			},
			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.CAFile).To(BeEquivalentTo("mock"))
			},
		},
		{
			name: "Sets CAFile for HTTPS using ca.crt and ignores caFile",
			URL:  "https://example.com",
			data: map[string][]byte{
				dataKeyCACert: []byte("mock"),
				dataKeyCAFile: []byte("ignored"),
			},
			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.CAFile).To(BeEquivalentTo("mock"))
			},
		},
		{
			name: "Sets transport from URL",
			URL:  "http://git@example.com",
			data: nil,
			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.Transport).To(Equal(HTTP))
			},
		},
		{
			name: "Sets user from URL",
			URL:  "http://example@example.com",
			data: map[string][]byte{
				dataKeyPassword: []byte("secret"),
			},

			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.Username).To(Equal("example"))
				g.Expect(opts.Password).To(Equal("secret"))
			},
		},
		{
			name: "Sets username from Secret over username from URL",
			URL:  "http://example@example.com",
			data: map[string][]byte{
				dataKeyUsername: []byte("user"),
				dataKeyPassword: []byte("secret"),
			},

			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.Username).To(Equal("user"))
				g.Expect(opts.Password).To(Equal("secret"))
			},
		},
		{
			name: "Sets username and password from URL only",
			URL:  "https://user:pass@example.com",
			data: nil,
			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.Username).To(Equal("user"))
				g.Expect(opts.Password).To(Equal("pass"))
			},
		},
		{
			name: "Validates options",
			URL:  "ssh://example.com",
			data: map[string][]byte{
				dataKeyIdentity: []byte(privateKeyFixture),
			},
			wantErr: "invalid 'ssh' auth option: 'known_hosts' is required",
		},
		{
			name: "Returns a minimal auth options without any data",
			URL:  "http://example.com",
			data: nil,
			wantFunc: func(g *WithT, opts *AuthOptions) {
				g.Expect(opts.Host).To(Equal("example.com"))
				g.Expect(opts.Transport).To(Equal(HTTP))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			url, err := url.Parse(tt.URL)
			g.Expect(err).ToNot(HaveOccurred())

			got, err := NewAuthOptions(*url, tt.data)
			if tt.wantErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr))
				g.Expect(got).To(BeNil())
				return
			}

			g.Expect(err).To(BeNil())
			if tt.wantFunc != nil {
				tt.wantFunc(g, got)
			}
		})
	}
}
