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
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
)

func TestProvider_GetGARAuthConfig(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		statusCode     int
		wantErr        bool
		wantAuthConfig authn.AuthConfig
		wantExpiry     time.Duration
	}{
		{
			name: "success",
			responseBody: `{
	"access_token": "access-token",
	"expires_in": 10,
	"token_type": "Bearer"
}`,
			statusCode: http.StatusOK,
			wantAuthConfig: authn.AuthConfig{
				Username: DefaultGARUsername,
				Password: "access-token",
			},
			wantExpiry: time.Second * 10,
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

			provider := NewProvider(WithTokenURL(srv.URL))
			auth, expiry, err := provider.GetGARAuthConfig(context.TODO())
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				if tt.statusCode == http.StatusOK {
					g.Expect(auth).To(Equal(tt.wantAuthConfig))
					g.Expect(expiry).To(Equal(tt.wantExpiry))
				}
			}
		})
	}
}
