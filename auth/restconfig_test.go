/*
Copyright 2025 The Flux authors

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

package auth_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/auth"
)

func TestParseClusterAddress(t *testing.T) {
	tests := []struct {
		address  string
		expected string
		err      string
	}{
		{
			address:  "https://example.com:443",
			expected: "https://example.com:443",
		},
		{
			address:  "example.com",
			expected: "https://example.com:443",
		},
		{
			address:  "EXAMPLE.COM:8080",
			expected: "https://example.com:8080",
		},
		{
			address:  "34.44.60.80",
			expected: "https://34.44.60.80:443",
		},
		{
			address: "",
			err:     "empty address",
		},
		{
			address: "------------\t",
			err:     "failed to parse Kubernetes API server address 'https://------------	':",
		},
		{
			address: "http://example.com:443",
			err:     "Kubernetes API server address 'http://example.com:443' must use https scheme",
		},
	}

	for _, tt := range tests {
		t.Run(strings.ReplaceAll(tt.address, "/", ""), func(t *testing.T) {
			g := NewWithT(t)

			address, err := auth.ParseClusterAddress(tt.address)

			if tt.err != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.err))
				g.Expect(address).To(BeEmpty())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(address).To(Equal(tt.expected))
			}
		})
	}
}

func TestGetRESTConfig(t *testing.T) {
	// TODO
}
