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
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/fluxcd/pkg/cache"
	"github.com/fluxcd/pkg/ssh"
	. "github.com/onsi/gomega"
)

func TestClient_Options(t *testing.T) {
	appID := "123"
	installationID := "456"
	kp, _ := ssh.GenerateKeyPair(ssh.RSA_4096)
	gitHubDefaultURL := "https://api.github.com"
	gitHubEnterpriseURL := "https://github.example.com/api/v3"
	proxy, _ := url.Parse("http://localhost:8080")

	tests := []struct {
		name    string
		opts    []OptFunc
		wantErr error
	}{
		{
			name: "Create new client",
			opts: []OptFunc{WithInstllationID(installationID), WithAppID(appID), WithPrivateKey(kp.PrivateKey)},
		},
		{
			name: "Create new client with proxy",
			opts: []OptFunc{WithInstllationID(installationID), WithAppID(appID), WithPrivateKey(kp.PrivateKey), WithProxyURL((proxy))},
		},
		{
			name: "Create new client with custom api url",
			opts: []OptFunc{WithAppBaseURL(gitHubEnterpriseURL), WithInstllationID(installationID), WithAppID(appID), WithPrivateKey(kp.PrivateKey)},
		},
		{
			name: "Create new client with app data",
			opts: []OptFunc{WithAppData(map[string][]byte{
				AppIDKey:             []byte(appID),
				AppInstallationIDKey: []byte(installationID),
				AppPrivateKey:        kp.PrivateKey,
			},
			)},
		},
		{
			name:    "Create new client with empty data",
			opts:    []OptFunc{WithAppData(map[string][]byte{})},
			wantErr: errors.New("app ID must be provided to use github app authentication"),
		},
		{
			name: "Create new client with app data with missing AppID Key",
			opts: []OptFunc{WithAppData(map[string][]byte{
				AppInstallationIDKey: []byte(installationID),
				AppPrivateKey:        kp.PrivateKey,
			},
			)},
			wantErr: errors.New("app ID must be provided to use github app authentication"),
		},
		{
			name: "Create new client with app data with missing AppInstallationID Key",
			opts: []OptFunc{WithAppData(map[string][]byte{
				AppIDKey:      []byte("123"),
				AppPrivateKey: kp.PrivateKey,
			},
			)},
			wantErr: errors.New("app installation ID must be provided to use github app authentication"),
		},
		{
			name: "Create new client with app data with missing private Key",
			opts: []OptFunc{WithAppData(map[string][]byte{
				AppIDKey:             []byte(appID),
				AppInstallationIDKey: []byte(installationID),
			},
			)},
			wantErr: errors.New("private key must be provided to use github app authentication"),
		},
		{
			name: "Create new client with invalid appID in app data",
			opts: []OptFunc{WithAppData(map[string][]byte{
				AppIDKey:             []byte("abc"),
				AppInstallationIDKey: []byte(installationID),
				AppPrivateKey:        kp.PrivateKey,
			},
			)},
			wantErr: errors.New("invalid app id, err: strconv.Atoi: parsing \"abc\": invalid syntax"),
		},
		{
			name: "Create new client with invalid installationID in app data",
			opts: []OptFunc{WithAppData(map[string][]byte{
				AppIDKey:             []byte(appID),
				AppInstallationIDKey: []byte("abc"),
				AppPrivateKey:        kp.PrivateKey,
			},
			)},
			wantErr: errors.New("invalid app installation id, err: strconv.Atoi: parsing \"abc\": invalid syntax"),
		},
		{
			name: "Create new client with invalid private key in app data",
			opts: []OptFunc{WithAppData(map[string][]byte{
				AppIDKey:             []byte(appID),
				AppInstallationIDKey: []byte(installationID),
				AppPrivateKey:        []byte("  "),
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
				g.Expect(client.appID).To(Equal(appID))
				g.Expect(client.installationID).To(Equal(installationID))
				g.Expect(client.privateKey).To(Equal(kp.PrivateKey))

				if client.apiURL != "" {
					g.Expect(client.apiURL).To(Equal(gitHubEnterpriseURL))
					g.Expect(client.ghTransport.BaseURL).To(Equal(gitHubEnterpriseURL))
				} else {
					g.Expect(client.ghTransport.BaseURL).To(Equal(gitHubDefaultURL))
				}
			}
		})
	}
}

func TestClient_GetToken(t *testing.T) {
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
				c := cache.NewTokenCache(1)
				c.GetOrSet(context.Background(), client.buildCacheKey(), func(context.Context) (cache.Token, error) {
					return &AppToken{
						Token:     "access-token",
						ExpiresAt: expiresAt,
					}, nil
				})
				client.cache = c
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
				WithAppBaseURL(srv.URL), WithInstllationID("123"), WithAppID("456"), WithPrivateKey(kp.PrivateKey),
			}
			opts = append(opts, tt.opts...)

			provider, err := New(opts...)
			g.Expect(err).ToNot(HaveOccurred())

			appToken, err := provider.GetToken(context.TODO())
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(appToken.Token).To(Equal(tt.wantAppToken.Token))
				g.Expect(appToken.ExpiresAt).To(Equal(tt.wantAppToken.ExpiresAt))
			}
		})
	}
}
