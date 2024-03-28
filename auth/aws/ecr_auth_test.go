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

package aws

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/google/go-containerregistry/pkg/authn"
	. "github.com/onsi/gomega"
)

const (
	testValidECRImage = "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo:v1"
)

func TestParseRegistry(t *testing.T) {
	tests := []struct {
		registry      string
		wantAccountID string
		wantRegion    string
		wantOK        bool
	}{
		{
			registry:      "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo:v1",
			wantAccountID: "012345678901",
			wantRegion:    "us-east-1",
			wantOK:        true,
		},
		{
			registry:      "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo",
			wantAccountID: "012345678901",
			wantRegion:    "us-east-1",
			wantOK:        true,
		},
		{
			registry:      "012345678901.dkr.ecr.us-east-1.amazonaws.com",
			wantAccountID: "012345678901",
			wantRegion:    "us-east-1",
			wantOK:        true,
		},
		{
			registry: "gcr.io/foo/bar:baz",
			wantOK:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.registry, func(t *testing.T) {
			g := NewWithT(t)

			accId, region, ok := ParseRegistry(tt.registry)
			g.Expect(ok).To(Equal(tt.wantOK), "unexpected OK")
			g.Expect(accId).To(Equal(tt.wantAccountID), "unexpected account IDs")
			g.Expect(region).To(Equal(tt.wantRegion), "unexpected regions")
		})
	}
}

func TestProvider_GetECRAuthConfig(t *testing.T) {
	expiresAt := time.Now().Add(time.Hour)
	tests := []struct {
		name           string
		responseBody   []byte
		statusCode     int
		wantErr        bool
		wantAuthConfig authn.AuthConfig
	}{
		{
			// NOTE: The authorizationToken is base64 encoded.
			name: "success",
			responseBody: []byte(`{
	"authorizationData": [
		{
			"authorizationToken": "c29tZS1rZXk6c29tZS1zZWNyZXQ=",
			"expiresAt": <expiresAt>
		}
	]
}`),
			statusCode: http.StatusOK,
			wantAuthConfig: authn.AuthConfig{
				Username: "some-key",
				Password: "some-secret",
			},
		},
		{
			name:       "fail",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name: "invalid token",
			responseBody: []byte(`{
	"authorizationData": [
		{
			"authorizationToken": "c29tZS10b2tlbg=="
		}
	]
}`),
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name: "invalid data",
			responseBody: []byte(`{
	"authorizationData": [
		{
			"foo": "bar"
		}
	]
}`),
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name:         "invalid response",
			responseBody: []byte(`{}`),
			statusCode:   http.StatusOK,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(strings.ReplaceAll(
					string(tt.responseBody), "<expiresAt>", fmt.Sprint(expiresAt.Unix())),
				))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			cfg := aws.NewConfig()
			cfg.EndpointResolverWithOptions = aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{URL: srv.URL}, nil
			})
			cfg.Credentials = credentials.NewStaticCredentialsProvider("x", "y", "z")

			provider := NewProvider(WithConfig(*cfg))
			auth, expiry, err := provider.GetECRAuthConfig(context.TODO(), "0123.dkr.ecr.us-east-1.amazonaws.com/foo:v1")
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
				if tt.statusCode == http.StatusOK {
					g.Expect(auth).To(Equal(tt.wantAuthConfig))
					g.Expect(time.Now().UTC().Add(expiry)).To(BeTemporally("~", expiresAt, time.Second))
				}
			}
		})
	}
}
