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
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/auth"
)

func TestGetRegistryFromArtifactRepository(t *testing.T) {
	for _, tt := range []struct {
		name               string
		artifactRepository string
		expectedRegistry   string
	}{
		{
			name:               "dot-less host with port",
			artifactRepository: "localhost:5000",
			expectedRegistry:   "localhost:5000",
		},
		{
			name:               "dot-less host without port",
			artifactRepository: "localhost",
			expectedRegistry:   "localhost",
		},
		{
			name:               "host with port",
			artifactRepository: "registry.io:5000",
			expectedRegistry:   "registry.io:5000",
		},
		{
			name:               "host without port",
			artifactRepository: "registry.io",
			expectedRegistry:   "registry.io",
		},
		{
			name:               "dot-less repo with port",
			artifactRepository: "localhost:5000/repo",
			expectedRegistry:   "localhost:5000",
		},
		{
			name:               "dot-less repo without port",
			artifactRepository: "localhost/repo",
			expectedRegistry:   "index.docker.io",
		},
		{
			name:               "repo with port",
			artifactRepository: "registry.io:5000/repo",
			expectedRegistry:   "registry.io:5000",
		},
		{
			name:               "repo without port",
			artifactRepository: "registry.io/repo",
			expectedRegistry:   "registry.io",
		},
		{
			name:               "tag",
			artifactRepository: "registry.io/repo:tag",
			expectedRegistry:   "registry.io",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			reg, err := auth.GetRegistryFromArtifactRepository(tt.artifactRepository)

			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(reg).To(Equal(tt.expectedRegistry))
		})
	}
}
