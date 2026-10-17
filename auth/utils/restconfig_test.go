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

package utils

import (
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"

	cryptov1 "github.com/fluxcd/pkg/apis/crypto/v1"
	"github.com/fluxcd/pkg/apis/meta"
)

func TestValidateKubeConfigGCPAudiences(t *testing.T) {
	g := NewWithT(t)

	cm := &corev1.ConfigMap{Data: map[string]string{
		meta.KubeConfigKeyProvider:  cryptov1.ProviderGCP,
		meta.KubeConfigKeyAudiences: "audience",
	}}

	// Controller-level GCP authentication ignores legacy audiences.
	_, err := validateKubeConfig(cm, cryptov1.ProviderGCP, nil)
	g.Expect(err).ToNot(HaveOccurred())

	// An explicit ServiceAccount uses object-level GCP workload identity,
	// where custom audiences are rejected.
	cm.Data[meta.KubeConfigKeyServiceAccountName] = "sa"
	_, err = validateKubeConfig(cm, cryptov1.ProviderGCP, &corev1.ServiceAccount{})
	g.Expect(err).To(MatchError(ContainSubstring("audiences are not supported")))

	// Other providers keep using the legacy audiences.
	_, err = validateKubeConfig(&corev1.ConfigMap{Data: map[string]string{
		meta.KubeConfigKeyAudiences: "audience",
	}}, cryptov1.ProviderGeneric, nil)
	g.Expect(err).ToNot(HaveOccurred())
}
