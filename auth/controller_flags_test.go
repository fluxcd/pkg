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
	"os"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/auth"
)

func TestSetDefaultServiceAccount(t *testing.T) {
	g := NewWithT(t)
	auth.SetDefaultServiceAccount("test-sa")
	t.Cleanup(func() {
		os.Unsetenv(auth.EnvDefaultServiceAccount)
	})
	g.Expect(os.Getenv(auth.EnvDefaultServiceAccount)).To(Equal("test-sa"))
}

func TestSetDefaultKubeConfigServiceAccount(t *testing.T) {
	g := NewWithT(t)
	auth.SetDefaultKubeConfigServiceAccount("test-kubeconfig-sa")
	t.Cleanup(func() {
		os.Unsetenv(auth.EnvDefaultKubeConfigServiceAccount)
	})
	g.Expect(os.Getenv(auth.EnvDefaultKubeConfigServiceAccount)).To(Equal("test-kubeconfig-sa"))
}

func TestSetDefaultDecryptionServiceAccount(t *testing.T) {
	g := NewWithT(t)
	auth.SetDefaultDecryptionServiceAccount("test-decryption-sa")
	t.Cleanup(func() {
		os.Unsetenv(auth.EnvDefaultDecryptionServiceAccount)
	})
	g.Expect(os.Getenv(auth.EnvDefaultDecryptionServiceAccount)).To(Equal("test-decryption-sa"))
}

func TestGetDefaultServiceAccount(t *testing.T) {
	t.Run("returns set value", func(t *testing.T) {
		g := NewWithT(t)
		t.Setenv(auth.EnvDefaultServiceAccount, "expected-sa")

		g.Expect(auth.GetDefaultServiceAccount()).To(Equal("expected-sa"))
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(auth.GetDefaultServiceAccount()).To(Equal(""))
	})
}

func TestGetDefaultKubeConfigServiceAccount(t *testing.T) {
	t.Run("returns set value", func(t *testing.T) {
		g := NewWithT(t)
		t.Setenv(auth.EnvDefaultKubeConfigServiceAccount, "expected-kubeconfig-sa")

		g.Expect(auth.GetDefaultKubeConfigServiceAccount()).To(Equal("expected-kubeconfig-sa"))
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(auth.GetDefaultKubeConfigServiceAccount()).To(Equal(""))
	})
}

func TestGetDefaultDecryptionServiceAccount(t *testing.T) {
	t.Run("returns set value", func(t *testing.T) {
		g := NewWithT(t)
		t.Setenv(auth.EnvDefaultDecryptionServiceAccount, "expected-decryption-sa")

		g.Expect(auth.GetDefaultDecryptionServiceAccount()).To(Equal("expected-decryption-sa"))
	})

	t.Run("returns empty when not set", func(t *testing.T) {
		g := NewWithT(t)

		g.Expect(auth.GetDefaultDecryptionServiceAccount()).To(Equal(""))
	})
}
