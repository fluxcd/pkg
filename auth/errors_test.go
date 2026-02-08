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
	"errors"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/auth"
)

func TestErrInvalidIdentityType(t *testing.T) {
	g := NewWithT(t)

	err := auth.ErrInvalidIdentityType(mockIdentity("want"), mockIdentity("got"))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid identity type"))
	g.Expect(err.Error()).To(ContainSubstring("auth_test.mockIdentity"))
}

func TestErrNoIdentityForOIDCImpersonation(t *testing.T) {
	g := NewWithT(t)

	g.Expect(auth.ErrNoIdentityForOIDCImpersonation).To(HaveOccurred())
	g.Expect(errors.Is(auth.ErrNoIdentityForOIDCImpersonation, auth.ErrNoIdentityForOIDCImpersonation)).To(BeTrue())
}

func TestErrNoAudienceForOIDCImpersonation(t *testing.T) {
	g := NewWithT(t)

	g.Expect(auth.ErrNoAudienceForOIDCImpersonation).To(HaveOccurred())
	g.Expect(errors.Is(auth.ErrNoAudienceForOIDCImpersonation, auth.ErrNoAudienceForOIDCImpersonation)).To(BeTrue())
}
