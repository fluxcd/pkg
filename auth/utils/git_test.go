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
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	authutils "github.com/fluxcd/pkg/auth/utils"
)

func TestGetGitCredentials(t *testing.T) {
	t.Run("azure", func(t *testing.T) {
		g := NewWithT(t)
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		p, err := authutils.GetGitCredentials(ctx, "azure")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).NotTo(ContainSubstring("does not support Git credentials"))
		g.Expect(p).To(BeNil())
	})

	t.Run("unknown provider", func(t *testing.T) {
		g := NewWithT(t)
		p, err := authutils.GetGitCredentials(context.Background(), "unknown")
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(Equal("provider 'unknown' does not support Git credentials"))
		g.Expect(p).To(BeNil())
	})
}
