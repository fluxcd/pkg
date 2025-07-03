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

package utils_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/auth"
	"github.com/fluxcd/pkg/auth/aws"
	"github.com/fluxcd/pkg/auth/azure"
	"github.com/fluxcd/pkg/auth/gcp"
	authutils "github.com/fluxcd/pkg/auth/utils"
)

func TestProviderByName(t *testing.T) {
	for _, tt := range []struct {
		name     string
		provider auth.Provider
	}{
		{
			name:     "azure",
			provider: azure.Provider{},
		},
		{
			name:     "aws",
			provider: aws.Provider{},
		},
		{
			name:     "gcp",
			provider: gcp.Provider{},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			p, err := authutils.ProviderByName(tt.name)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(p).To(Equal(tt.provider))
		})
	}

	t.Run("unknown provider", func(t *testing.T) {
		g := NewWithT(t)
		p, err := authutils.ProviderByName("unknown")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("provider 'unknown' not implemented"))
		g.Expect(p).To(BeNil())
	})
}
