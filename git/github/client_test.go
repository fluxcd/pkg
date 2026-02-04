/*
Copyright 2024 The Flux authors

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

package github

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/ssh"
)

func TestClient_Options(t *testing.T) {
	appID := "123"
	installationID := "456"
	kp, _ := ssh.GenerateKeyPair(ssh.RSA_4096)
	gitHubEnterpriseURL := "https://github.example.com/api/v3"
	proxy, _ := url.Parse("http://localhost:8080")

	tests := []struct {
		name                  string
		opts                  []OptFunc
		wantErr               error
		wantInstallationID    int64
		wantInstallationOwner string
	}{
		{
			name: "Create new client with proxy",
			opts: []OptFunc{
				WithAppData(map[string][]byte{
					KeyAppID:             []byte(appID),
					KeyAppInstallationID: []byte(installationID),
					KeyAppPrivateKey:     kp.PrivateKey,
				}),
				WithProxyURL(proxy),
			},
			wantInstallationID: 456,
		},
		{
			name: "Create new client",
			opts: []OptFunc{WithAppData(map[string][]byte{
				KeyAppID:             []byte(appID),
				KeyAppInstallationID: []byte(installationID),
				KeyAppPrivateKey:     kp.PrivateKey,
			})},
			wantInstallationID: 456,
		},
		{
			name: "Create new client with custom api url",
			opts: []OptFunc{WithAppData(map[string][]byte{
				KeyAppID:             []byte(appID),
				KeyAppInstallationID: []byte(installationID),
				KeyAppBaseURL:        []byte(gitHubEnterpriseURL),
				KeyAppPrivateKey:     kp.PrivateKey,
			})},
			wantInstallationID: 456,
		},
		{
			name: "Create new client with installation owner",
			opts: []OptFunc{WithAppData(map[string][]byte{
				KeyAppID:                []byte(appID),
				KeyAppInstallationOwner: []byte("my-org"),
				KeyAppPrivateKey:        kp.PrivateKey,
			})},
			wantInstallationOwner: "my-org",
		},
		{
			name: "Create new client with both installation owner and ID",
			opts: []OptFunc{WithAppData(map[string][]byte{
				KeyAppID:                []byte(appID),
				KeyAppInstallationOwner: []byte("my-org"),
				KeyAppInstallationID:    []byte(installationID),
				KeyAppPrivateKey:        kp.PrivateKey,
			})},
			wantErr: errors.New("only one of app installation owner or ID must be provided to use github app authentication"),
		},
		{
			name:    "Create new client with empty data",
			opts:    []OptFunc{WithAppData(map[string][]byte{})},
			wantErr: errors.New("app ID must be provided to use github app authentication"),
		},
		{
			name: "Create new client with app data with missing AppID Key",
			opts: []OptFunc{WithAppData(map[string][]byte{
				KeyAppInstallationID: []byte(installationID),
				KeyAppPrivateKey:     kp.PrivateKey,
			},
			)},
			wantErr: errors.New("app ID must be provided to use github app authentication"),
		},
		{
			name: "Create new client with app data with missing AppInstallationID Key",
			opts: []OptFunc{WithAppData(map[string][]byte{
				KeyAppID:         []byte("123"),
				KeyAppPrivateKey: kp.PrivateKey,
			},
			)},
			wantErr: errors.New("app installation owner or ID must be provided to use github app authentication"),
		},
		{
			name: "Create new client with app data with missing private Key",
			opts: []OptFunc{WithAppData(map[string][]byte{
				KeyAppID:             []byte(appID),
				KeyAppInstallationID: []byte(installationID),
			},
			)},
			wantErr: errors.New("private key must be provided to use github app authentication"),
		},
		{
			name: "Create new client with invalid appID in app data",
			opts: []OptFunc{WithAppData(map[string][]byte{
				KeyAppID:             []byte("abc"),
				KeyAppInstallationID: []byte(installationID),
				KeyAppPrivateKey:     kp.PrivateKey,
			},
			)},
			wantErr: errors.New("app ID must be provided to use github app authentication"),
		},
		{
			name: "Create new client with invalid installationID in app data",
			opts: []OptFunc{WithAppData(map[string][]byte{
				KeyAppID:             []byte(appID),
				KeyAppInstallationID: []byte("abc"),
				KeyAppPrivateKey:     kp.PrivateKey,
			},
			)},
			wantErr: errors.New("app installation owner or ID must be provided to use github app authentication"),
		},
		{
			name: "Create new client with invalid private key in app data",
			opts: []OptFunc{WithAppData(map[string][]byte{
				KeyAppID:             []byte(appID),
				KeyAppInstallationID: []byte(installationID),
				KeyAppPrivateKey:     []byte("  "),
			},
			)},
			wantErr: errors.New("could not parse private key: invalid key: Key must be a PEM encoded PKCS1 or PKCS8 key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			opts := tt.opts

			client, err := New(opts...)
			if tt.wantErr != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.wantErr.Error()))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(client.appID).To(Equal(int64(123)))
				g.Expect(client.installationID).To(Equal(tt.wantInstallationID))
				g.Expect(client.installationOwner).To(Equal(tt.wantInstallationOwner))
				g.Expect(client.privateKey).To(Equal(kp.PrivateKey))

				if client.apiURL != "" {
					g.Expect(client.apiURL).To(Equal(gitHubEnterpriseURL))
				}
			}
		})
	}
}

func TestClient_GetCredentials(t *testing.T) {
	g := NewWithT(t)

	expiresAt := time.Now().UTC().Add(time.Hour)
	tests := []struct {
		name         string
		opts         []OptFunc
		accessToken  *AppToken
		statusCode   int
		wantErr      bool
		wantAppToken *AppToken
	}{
		{
			name: "Get valid token",
			accessToken: &AppToken{
				Token:     "access-token",
				ExpiresAt: expiresAt,
			},
			statusCode: http.StatusOK,
			wantAppToken: &AppToken{
				Token:     "access-token",
				ExpiresAt: expiresAt,
			},
		},
		{
			name: "Get cached token",
			opts: []OptFunc{func(client *Client) {
				c, err := cache.NewTokenCache(1)
				g.Expect(err).NotTo(HaveOccurred())
				_, ok, err := c.GetOrSet(context.Background(), client.buildCacheKey(), func(context.Context) (cache.Token, error) {
					return &AppToken{
						Token:     "access-token",
						ExpiresAt: expiresAt,
					}, nil
				})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(ok).To(BeFalse())
				WithCache(c, "", "", "", "")(client)
			}},
			statusCode: http.StatusInternalServerError, // error status code to make the test fail if the token is not cached
			wantAppToken: &AppToken{
				Token:     "access-token",
				ExpiresAt: expiresAt,
			},
		},
		{
			name:       "Failure in getting token",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				var response []byte
				var err error
				if tt.accessToken != nil {
					response, err = json.Marshal(tt.accessToken)
					g.Expect(err).ToNot(HaveOccurred())
				}
				w.Write(response)
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			kp, err := ssh.GenerateKeyPair(ssh.RSA_4096)
			g.Expect(err).ToNot(HaveOccurred())
			opts := []OptFunc{
				WithAppData(map[string][]byte{
					KeyAppID:             []byte("123"),
					KeyAppInstallationID: []byte("456"),
					KeyAppBaseURL:        []byte(srv.URL),
					KeyAppPrivateKey:     kp.PrivateKey,
				}),
			}
			opts = append(opts, tt.opts...)

			username, password, err := GetCredentials(context.Background(), opts...)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(username).To(Equal("x-access-token"))
				g.Expect(password).To(Equal(tt.wantAppToken.Token))
			}
		})
	}
}

func TestClient_TLS_RootCA(t *testing.T) {
	g := NewWithT(t)

	// spin up a TLS server with a self-signed cert
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		tok := &AppToken{
			Token:     "enterprise-token",
			ExpiresAt: time.Now().Add(time.Hour),
		}
		_ = json.NewEncoder(w).Encode(tok)
	})
	srv := httptest.NewTLSServer(handler)
	defer srv.Close()

	// generate a dummy GitHub App keypair
	kp, err := ssh.GenerateKeyPair(ssh.RSA_4096)
	g.Expect(err).NotTo(HaveOccurred())

	opts := []OptFunc{
		WithAppData(map[string][]byte{
			KeyAppID:             []byte("123"),
			KeyAppInstallationID: []byte("456"),
			KeyAppPrivateKey:     kp.PrivateKey,
			KeyAppBaseURL:        []byte(srv.URL),
		}),
	}

	t.Run("it should error out if a Root CA is not provided", func(t *testing.T) {
		g := NewWithT(t)
		// with no TLSConfig, system roots won’t trust our server’s cert
		_, _, err := GetCredentials(context.Background(), opts...)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("certificate signed by unknown authority"))
	})

	t.Run("it should succeed when Root CA is provided", func(t *testing.T) {
		g := NewWithT(t)
		// create a cert pool with server cert
		certPool := x509.NewCertPool()
		certPool.AddCert(srv.Certificate())

		opts := append(opts,
			WithTLSConfig(&tls.Config{RootCAs: certPool}),
		)
		user, pass, err := GetCredentials(context.Background(), opts...)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(user).To(Equal(AccessTokenUsername))
		g.Expect(pass).To(Equal("enterprise-token"))
	})
}

func TestClient_WithAppSlugReflection(t *testing.T) {
	g := NewWithT(t)

	kp, err := ssh.GenerateKeyPair(ssh.RSA_4096)
	g.Expect(err).ToNot(HaveOccurred())

	t.Run("reflectSlug is false by default", func(t *testing.T) {
		g := NewWithT(t)
		client, err := New(
			WithAppData(map[string][]byte{
				KeyAppID:             []byte("123"),
				KeyAppInstallationID: []byte("456"),
				KeyAppPrivateKey:     kp.PrivateKey,
			}),
		)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(client.reflectSlug).To(BeFalse())
	})

	t.Run("reflectSlug is true when WithAppSlugReflection is used", func(t *testing.T) {
		g := NewWithT(t)
		client, err := New(
			WithAppData(map[string][]byte{
				KeyAppID:             []byte("123"),
				KeyAppInstallationID: []byte("456"),
				KeyAppPrivateKey:     kp.PrivateKey,
			}),
			WithAppSlugReflection(),
		)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(client.reflectSlug).To(BeTrue())
	})
}

func TestClient_GetToken_WithSlugReflection(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour)

	tests := []struct {
		name        string
		reflectSlug bool
		appSlug     string
		wantSlug    string
	}{
		{
			name:        "Slug is not reflected when disabled",
			reflectSlug: false,
			appSlug:     "my-app",
			wantSlug:    "",
		},
		{
			name:        "Slug is reflected when enabled",
			reflectSlug: true,
			appSlug:     "my-app",
			wantSlug:    "my-app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				var response []byte
				var err error

				switch r.URL.Path {
				case "/app":
					response, err = json.Marshal(map[string]interface{}{
						"slug": tt.appSlug,
					})
					g.Expect(err).ToNot(HaveOccurred())
				case "/app/installations/456/access_tokens":
					response, err = json.Marshal(&AppToken{
						Token:     "access-token",
						ExpiresAt: expiresAt,
					})
					g.Expect(err).ToNot(HaveOccurred())
				default:
					w.WriteHeader(http.StatusNotFound)
					return
				}

				w.Write(response)
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			kp, err := ssh.GenerateKeyPair(ssh.RSA_4096)
			g.Expect(err).ToNot(HaveOccurred())

			opts := []OptFunc{
				WithAppData(map[string][]byte{
					KeyAppID:             []byte("123"),
					KeyAppInstallationID: []byte("456"),
					KeyAppBaseURL:        []byte(srv.URL),
					KeyAppPrivateKey:     kp.PrivateKey,
				}),
			}
			if tt.reflectSlug {
				opts = append(opts, WithAppSlugReflection())
			}

			client, err := New(opts...)
			g.Expect(err).ToNot(HaveOccurred())

			token, err := client.GetToken(context.Background())
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(token.Token).To(Equal("access-token"))
			g.Expect(token.Slug).To(Equal(tt.wantSlug))
		})
	}
}

func TestClient_CacheKey_WithSlugReflection(t *testing.T) {
	g := NewWithT(t)

	kp, err := ssh.GenerateKeyPair(ssh.RSA_4096)
	g.Expect(err).ToNot(HaveOccurred())

	baseOpts := []OptFunc{
		WithAppData(map[string][]byte{
			KeyAppID:             []byte("123"),
			KeyAppInstallationID: []byte("456"),
			KeyAppPrivateKey:     kp.PrivateKey,
		}),
	}

	clientWithoutSlugReflection, err := New(baseOpts...)
	g.Expect(err).ToNot(HaveOccurred())

	clientWithSlugReflection, err := New(append(baseOpts, WithAppSlugReflection())...)
	g.Expect(err).ToNot(HaveOccurred())

	// Cache keys should be different when reflectSlug differs
	keyWithout := clientWithoutSlugReflection.buildCacheKey()
	keyWith := clientWithSlugReflection.buildCacheKey()
	g.Expect(keyWithout).ToNot(Equal(keyWith))
}

func TestClient_GetCredentials_InstallationOwner(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour)

	tests := []struct {
		name              string
		installationOwner string
		orgStatus         int
		userStatus        int
		wantErr           bool
		wantErrContains   string
	}{
		{
			name:              "Lookup org installation",
			installationOwner: "my-org",
			orgStatus:         http.StatusOK,
			userStatus:        http.StatusNotFound,
		},
		{
			name:              "Fallback to user installation",
			installationOwner: "my-user",
			orgStatus:         http.StatusNotFound,
			userStatus:        http.StatusOK,
		},
		{
			name:              "Both lookups fail",
			installationOwner: "unknown",
			orgStatus:         http.StatusNotFound,
			userStatus:        http.StatusNotFound,
			wantErr:           true,
			wantErrContains:   "failed to find organization installation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			handler := func(w http.ResponseWriter, r *http.Request) {
				var response []byte
				var err error

				switch r.URL.Path {
				case "/orgs/" + tt.installationOwner + "/installation":
					w.WriteHeader(tt.orgStatus)
					if tt.orgStatus == http.StatusOK {
						response, err = json.Marshal(map[string]interface{}{"id": 789})
						g.Expect(err).ToNot(HaveOccurred())
					}
				case "/users/" + tt.installationOwner + "/installation":
					w.WriteHeader(tt.userStatus)
					if tt.userStatus == http.StatusOK {
						response, err = json.Marshal(map[string]interface{}{"id": 789})
						g.Expect(err).ToNot(HaveOccurred())
					}
				case "/app/installations/789/access_tokens":
					w.WriteHeader(http.StatusOK)
					response, err = json.Marshal(&AppToken{
						Token:     "access-token",
						ExpiresAt: expiresAt,
					})
					g.Expect(err).ToNot(HaveOccurred())
				default:
					w.WriteHeader(http.StatusNotFound)
				}

				w.Write(response)
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			kp, err := ssh.GenerateKeyPair(ssh.RSA_4096)
			g.Expect(err).ToNot(HaveOccurred())
			opts := []OptFunc{
				WithAppData(map[string][]byte{
					KeyAppID:                []byte("123"),
					KeyAppInstallationOwner: []byte(tt.installationOwner),
					KeyAppBaseURL:           []byte(srv.URL),
					KeyAppPrivateKey:        kp.PrivateKey,
				}),
			}

			username, password, err := GetCredentials(context.Background(), opts...)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				if tt.wantErrContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.wantErrContains))
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(username).To(Equal("x-access-token"))
				g.Expect(password).To(Equal("access-token"))
			}
		})
	}
}
