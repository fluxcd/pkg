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
	"strings"
	"testing"
)

func TestParseClusterAddress(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
		wantErr string
	}{
		{
			name:    "https with port",
			address: "https://example.com:443",
			want:    "https://example.com:443",
		},
		{
			name:    "host without scheme",
			address: "example.com",
			want:    "https://example.com:443",
		},
		{
			name:    "host with port and uppercase",
			address: "EXAMPLE.COM:8080",
			want:    "https://example.com:8080",
		},
		{
			name:    "ip without port",
			address: "34.44.60.80",
			want:    "https://34.44.60.80:443",
		},
		{
			name:    "path is dropped",
			address: "https://example.com/k8s/clusters/c-m-abc123",
			want:    "https://example.com:443",
		},
		{
			name:    "empty address",
			address: "",
			wantErr: "empty address",
		},
		{
			name:    "invalid address",
			address: "------------\t",
			wantErr: "failed to parse Kubernetes API server address 'https://------------\t':",
		},
		{
			name:    "non-https scheme",
			address: "http://example.com:443",
			wantErr: "Kubernetes API server address 'http://example.com:443' must use https scheme",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseClusterAddress(tt.address)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got error %v, want containing %q", err, tt.wantErr)
				}
				if got != "" {
					t.Fatalf("got %q, want empty", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseClusterURL(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    string
		wantErr string
	}{
		{
			name:    "no path",
			address: "https://example.com",
			want:    "https://example.com:443",
		},
		{
			name:    "root path",
			address: "https://example.com/",
			want:    "https://example.com:443",
		},
		{
			name:    "path",
			address: "https://example.com/clusters/prod-1",
			want:    "https://example.com:443/clusters/prod-1",
		},
		{
			name:    "port, uppercase host, path and trailing slash",
			address: "https://Rancher.example.com:8443/k8s/clusters/c-m-abc123/",
			want:    "https://rancher.example.com:8443/k8s/clusters/c-m-abc123",
		},
		{
			name:    "multiple trailing slashes",
			address: "https://example.com/clusters/prod-1//",
			want:    "https://example.com:443/clusters/prod-1",
		},
		{
			name:    "path without scheme",
			address: "example.com:6443/clusters/prod-1",
			want:    "https://example.com:6443/clusters/prod-1",
		},
		{
			name:    "query and fragment are dropped",
			address: "https://example.com/clusters/prod-1?foo=bar#baz",
			want:    "https://example.com:443/clusters/prod-1",
		},
		{
			name:    "empty address",
			address: "",
			wantErr: "empty address",
		},
		{
			name:    "non-https scheme",
			address: "http://example.com/clusters/prod-1",
			wantErr: "Kubernetes API server address 'http://example.com/clusters/prod-1' must use https scheme",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseClusterURL(tt.address)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("got error %v, want containing %q", err, tt.wantErr)
				}
				if got != "" {
					t.Fatalf("got %q, want empty", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
