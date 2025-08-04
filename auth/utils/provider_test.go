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
	"github.com/fluxcd/pkg/auth/generic"
	authutils "github.com/fluxcd/pkg/auth/utils"
)

func TestProviderByName(t *testing.T) {
	t.Run("sts providers", func(t *testing.T) {
		for _, tt := range []struct {
			name     string
			provider any
		}{
			{
				name:     azure.ProviderName,
				provider: azure.Provider{},
			},
			{
				name:     aws.ProviderName,
				provider: aws.Provider{},
			},
			{
				name:     gcp.ProviderName,
				provider: gcp.NewProvider(),
			},
			{
				name:     generic.ProviderName,
				provider: generic.Provider{},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				g := NewWithT(t)
				p, err := authutils.ProviderByName[auth.Provider](tt.name)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(p).To(Equal(tt.provider))
			})
		}
	})

	t.Run("registry providers", func(t *testing.T) {
		for _, tt := range []struct {
			name     string
			provider any
		}{
			{
				name:     azure.ProviderName,
				provider: azure.Provider{},
			},
			{
				name:     aws.ProviderName,
				provider: aws.Provider{},
			},
			{
				name:     gcp.ProviderName,
				provider: gcp.NewProvider(),
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				g := NewWithT(t)
				p, err := authutils.ProviderByName[auth.ArtifactRegistryCredentialsProvider](tt.name)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(p).To(Equal(tt.provider))
			})
		}

		t.Run("generic provider", func(t *testing.T) {
			g := NewWithT(t)
			p, err := authutils.ProviderByName[auth.ArtifactRegistryCredentialsProvider](generic.ProviderName)
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("does not implement the expected interface"))
			g.Expect(p).To(BeNil())
		})
	})

	t.Run("restconfig providers", func(t *testing.T) {
		for _, tt := range []struct {
			name     string
			provider any
		}{
			{
				name:     azure.ProviderName,
				provider: azure.Provider{},
			},
			{
				name:     aws.ProviderName,
				provider: aws.Provider{},
			},
			{
				name:     gcp.ProviderName,
				provider: gcp.NewProvider(),
			},
			{
				name:     generic.ProviderName,
				provider: generic.Provider{},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				g := NewWithT(t)
				p, err := authutils.ProviderByName[auth.RESTConfigProvider](tt.name)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(p).To(Equal(tt.provider))
			})
		}
	})

	t.Run("errors", func(t *testing.T) {
		type iface interface{ foo() }

		for _, tt := range []struct {
			name     string
			provider any
		}{
			{
				name:     azure.ProviderName,
				provider: azure.Provider{},
			},
			{
				name:     aws.ProviderName,
				provider: aws.Provider{},
			},
			{
				name:     gcp.ProviderName,
				provider: gcp.NewProvider(),
			},
			{
				name:     generic.ProviderName,
				provider: generic.Provider{},
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				g := NewWithT(t)
				p, err := authutils.ProviderByName[iface](tt.name)
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring("does not implement the expected interface"))
				g.Expect(p).To(BeNil())
			})
		}

		t.Run("unknown provider", func(t *testing.T) {
			g := NewWithT(t)
			p, err := authutils.ProviderByName[iface]("unknown")
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(Equal("provider 'unknown' not implemented"))
			g.Expect(p).To(BeNil())
		})
	})
}
