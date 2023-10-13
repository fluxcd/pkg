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

package gcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetServiceAccountToken(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		statusCode   int
		wantErr      bool
		wantSAToken  ServiceAccountToken
	}{
		{
			name: "success",
			responseBody: `{
	"access_token": "access-token",
	"expires_in": 10,
	"token_type": "Bearer"
}`,
			statusCode: http.StatusOK,
			wantSAToken: ServiceAccountToken{
				AccessToken: "access-token",
				ExpiresIn:   10,
				TokenType:   "Bearer",
			},
		},
		{
			name:       "fail",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:         "invalid response",
			responseBody: "foo",
			statusCode:   http.StatusOK,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			provider := NewProvider(WithTokenURL(srv.URL))
			saToken, err := provider.GetServiceAccountToken(context.TODO())
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				if tt.statusCode == http.StatusOK {
					g.Expect(*saToken).To(Equal(tt.wantSAToken))
				}
			}
		})
	}
}

func TestGetServiceAccountEmail(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		statusCode   int
		wantErr      bool
		wantEmail    string
	}{
		{
			name:         "success",
			responseBody: "git@fluxcd.com",
			statusCode:   http.StatusOK,
			wantEmail:    "git@fluxcd.com",
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
				w.Write([]byte(tt.responseBody))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			provider := NewProvider(WithEmailURL(srv.URL))
			email, err := provider.GetServiceAccountEmail(context.TODO())
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				if tt.statusCode == http.StatusOK {
					g.Expect(email).To(Equal(tt.wantEmail))
				}
			}
		})
	}
}
