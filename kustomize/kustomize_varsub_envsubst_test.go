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

package kustomize_test

import (
	"os"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/kustomize"
)

func TestSubstituteEnvVariables(t *testing.T) {
	g := NewWithT(t)

	t.Setenv("APP_NAME", "myapp")

	input, err := os.ReadFile("./testdata/varsub_env_input.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	expected, err := os.ReadFile("./testdata/varsub_env_expected.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	result, err := kustomize.SubstituteEnvVariables(string(input), os.LookupEnv)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result).To(Equal(string(expected)))
}

func TestSubstituteEnvVariables_StrictError(t *testing.T) {
	g := NewWithT(t)

	// APP_NAME is not set, so the enabled resource should fail.
	input, err := os.ReadFile("./testdata/varsub_env_input.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	_, err = kustomize.SubstituteEnvVariables(string(input), os.LookupEnv)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("variable not set"))
}
