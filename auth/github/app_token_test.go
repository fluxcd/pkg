/*
Copyright 2023 The Flux authors

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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

type accessToken struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

var count int

func TestProvider_GetAppToken(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour)
	tests := []struct {
		name         string
		accessToken  *accessToken
		transport    http.RoundTripper
		statusCode   int
		wantErr      bool
		wantAppToken *AppToken
	}{
		{
			name: "success with default transport",
			accessToken: &accessToken{
				Token:     "access-token",
				ExpiresAt: expiresAt,
			},
			statusCode: http.StatusOK,
			wantAppToken: &AppToken{
				Token:     "access-token",
				ExpiresIn: time.Hour,
			},
		},
		{
			name: "success with custom transport",
			accessToken: &accessToken{
				Token:     "access-token",
				ExpiresAt: expiresAt,
			},
			statusCode: http.StatusOK,
			transport:  &testTransport{},
			wantAppToken: &AppToken{
				Token:     "access-token",
				ExpiresIn: time.Hour,
			},
		},
		{
			name:       "fail",
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

			pk, err := createPrivateKey()
			g.Expect(err).ToNot(HaveOccurred())
			opts := []ProviderOptFunc{
				WithApiURL(srv.URL), WithInstllationID(127), WithAppID(300), WithPrivateKey(pk),
			}
			if tt.transport != nil {
				opts = append(opts, WithTransport(tt.transport))
			}

			provider, err := NewProvider(opts...)
			g.Expect(err).ToNot(HaveOccurred())

			appToken, err := provider.GetAppToken(context.TODO())
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(appToken.Token).To(Equal(tt.wantAppToken.Token))
				g.Expect(appToken.ExpiresIn.Round(time.Hour)).To(Equal(tt.wantAppToken.ExpiresIn))
				if tt.transport != nil {
					g.Expect(count).To(BeNumerically(">", 0))
				}
			}
		})
	}
}

type testTransport struct{}

func (tt *testTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	count += 1
	return http.DefaultTransport.RoundTrip(req)
}

func createPrivateKey() ([]byte, error) {
	privatekey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	var privateKeyBytes []byte = x509.MarshalPKCS1PrivateKey(privatekey)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}

	pk := pem.EncodeToMemory(privateKeyBlock)
	return pk, nil
}
