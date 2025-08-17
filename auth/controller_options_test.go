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

func TestInconsistentObjectLevelConfiguration(t *testing.T) {
	tests := []struct {
		name                            string
		featureGateEnabled              bool
		defaultServiceAccount           string
		defaultKubeConfigServiceAccount string
		defaultDecryptionServiceAccount string
		expectInconsistent              bool
	}{
		{
			name:               "feature gate enabled, no default service accounts",
			featureGateEnabled: true,
			expectInconsistent: false,
		},
		{
			name:                  "feature gate enabled, default service account set",
			featureGateEnabled:    true,
			defaultServiceAccount: "test-sa",
			expectInconsistent:    false,
		},
		{
			name:                            "feature gate enabled, default kubeconfig service account set",
			featureGateEnabled:              true,
			defaultKubeConfigServiceAccount: "test-kubeconfig-sa",
			expectInconsistent:              false,
		},
		{
			name:                            "feature gate enabled, default decryption service account set",
			featureGateEnabled:              true,
			defaultDecryptionServiceAccount: "test-decryption-sa",
			expectInconsistent:              false,
		},
		{
			name:               "feature gate disabled, no default service accounts",
			featureGateEnabled: false,
			expectInconsistent: false,
		},
		{
			name:                  "feature gate disabled, default service account set",
			featureGateEnabled:    false,
			defaultServiceAccount: "test-sa",
			expectInconsistent:    true,
		},
		{
			name:                            "feature gate disabled, default kubeconfig service account set",
			featureGateEnabled:              false,
			defaultKubeConfigServiceAccount: "test-kubeconfig-sa",
			expectInconsistent:              true,
		},
		{
			name:                            "feature gate disabled, default decryption service account set",
			featureGateEnabled:              false,
			defaultDecryptionServiceAccount: "test-decryption-sa",
			expectInconsistent:              true,
		},
		{
			name:                            "feature gate disabled, all default service accounts set",
			featureGateEnabled:              false,
			defaultServiceAccount:           "test-sa",
			defaultKubeConfigServiceAccount: "test-kubeconfig-sa",
			defaultDecryptionServiceAccount: "test-decryption-sa",
			expectInconsistent:              true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			if tt.featureGateEnabled {
				auth.EnableObjectLevelWorkloadIdentity()
			}

			auth.SetDefaultServiceAccount(tt.defaultServiceAccount)
			auth.SetDefaultKubeConfigServiceAccount(tt.defaultKubeConfigServiceAccount)
			auth.SetDefaultDecryptionServiceAccount(tt.defaultDecryptionServiceAccount)

			t.Cleanup(func() {
				auth.SetDefaultServiceAccount("")
				auth.SetDefaultKubeConfigServiceAccount("")
				auth.SetDefaultDecryptionServiceAccount("")
				auth.DisableObjectLevelWorkloadIdentity()
			})

			result := auth.InconsistentObjectLevelConfiguration()
			g.Expect(result).To(Equal(tt.expectInconsistent))
		})
	}
}
