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

package utils_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"

	authutils "github.com/fluxcd/pkg/auth/utils"
)

func TestGetArtifactRegistryCredentials(t *testing.T) {
	t.Run("credentials are fetched lazily", func(t *testing.T) {
		g := NewWithT(t)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Force the context to be canceled to verify that credentials are fetched lazily.

		artifactRepository := "012345678901.dkr.ecr.us-east-1.amazonaws.com/repo"
		authenticator, err := authutils.GetArtifactRegistryCredentials(ctx, "aws", artifactRepository)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(authenticator).NotTo(BeNil())

		authConfig, err := authenticator.Authorization()
		g.Expect(err).To(HaveOccurred())
		g.Expect(err).To(MatchError(ContainSubstring("context canceled")))
		g.Expect(authConfig).To(BeNil())
	})
}
