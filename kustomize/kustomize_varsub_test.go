/*
Copyright 2022 The Flux authors

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
	"context"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	"github.com/fluxcd/pkg/kustomize"
)

func TestKustomization_Varsub(t *testing.T) {
	g := NewWithT(t)

	// Create a kustomization file with varsub
	yamlKus, err := os.ReadFile("./testdata/kustomization_varsub.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	clientObjects, err := readYamlObjects(strings.NewReader(string(yamlKus)))
	g.Expect(err).NotTo(HaveOccurred())

	// Create ns
	err = createObjectFile(kubeClient, "./testdata/ns.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	// Create configmap
	err = createObjectFile(kubeClient, "./testdata/configmap.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	// Get resource from directory
	fs := filesys.MakeFsOnDisk()
	resMap, err := kustomize.Build(fs, "./testdata/resources/")
	g.Expect(err).NotTo(HaveOccurred())
	for _, res := range resMap.Resources() {
		outRes, err := kustomize.SubstituteVariables(context.Background(),
			kubeClient, clientObjects[0], res)
		g.Expect(err).NotTo(HaveOccurred())

		if outRes != nil {
			_, err = resMap.Replace(res)
			g.Expect(err).NotTo(HaveOccurred())
		}
	}

	// Check that the resource has been substituted
	resources, err := resMap.AsYaml()
	g.Expect(err).NotTo(HaveOccurred())

	//load expected result
	expected, err := os.ReadFile("./testdata/kustomization_varsub_expected.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(string(resources)).To(Equal(string(expected)))

	// Test with strict mode on
	strictMapRes, err := kustomize.Build(fs, "./testdata/varsubstrict/")
	g.Expect(err).NotTo(HaveOccurred())
	_, err = kustomize.SubstituteVariables(context.Background(),
		kubeClient, clientObjects[0], strictMapRes.Resources()[0], kustomize.SubstituteWithStrict(true))
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("variable not set"))

	// Test with strict mode off
	_, err = kustomize.SubstituteVariables(context.Background(),
		kubeClient, clientObjects[0], strictMapRes.Resources()[0], kustomize.SubstituteWithStrict(false))
	g.Expect(err).ToNot(HaveOccurred())
}

func TestKustomization_Varsub_Always(t *testing.T) {
	// Load a Flux Kustomization that declares no substitution vars
	// (no postBuild.substitute nor postBuild.substituteFrom).
	yamlKus, err := os.ReadFile("./testdata/kustomization_varsub_novars.yaml")
	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())

	clientObjects, err := readYamlObjects(strings.NewReader(string(yamlKus)))
	g.Expect(err).NotTo(HaveOccurred())

	fs := filesys.MakeFsOnDisk()

	// build a fresh resource for each sub-test, as SubstituteVariables mutates it in place.
	buildResource := func() *resource.Resource {
		resMap, err := kustomize.Build(fs, "./testdata/varsubalways/")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(resMap.Resources()).To(HaveLen(1))
		return resMap.Resources()[0]
	}

	t.Run("disabled: default values are not substituted", func(t *testing.T) {
		g := NewWithT(t)

		res := buildResource()
		outRes, err := kustomize.SubstituteVariables(context.Background(),
			kubeClient, clientObjects[0], res)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(outRes).NotTo(BeNil())

		yml, err := outRes.AsYAML()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(string(yml)).To(ContainSubstring("${cluster_env:=dev}"))
		g.Expect(string(yml)).To(ContainSubstring("${cluster_region:=eu-central-1}"))
	})

	t.Run("enabled: default values are substituted", func(t *testing.T) {
		g := NewWithT(t)

		res := buildResource()
		outRes, err := kustomize.SubstituteVariables(context.Background(),
			kubeClient, clientObjects[0], res, kustomize.SubstituteWithAlways(true))
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(outRes).NotTo(BeNil())

		yml, err := outRes.AsYAML()
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(string(yml)).NotTo(ContainSubstring("${"))
		g.Expect(string(yml)).To(ContainSubstring("environment: dev"))
		g.Expect(string(yml)).To(ContainSubstring("region: eu-central-1"))
	})
}

// TestKustomization_Varsub_StrictEmptyValues verifies that, with strict mode
// enabled, variables explicitly set to the empty string are accepted (and
// substituted as empty) instead of being treated as unset, regardless of the
// source: the inline substitute map, a ConfigMap, or a Secret.
func TestKustomization_Varsub_StrictEmptyValues(t *testing.T) {
	g := NewWithT(t)

	// Flux Kustomization with an inline empty var plus substituteFrom a
	// ConfigMap and a Secret that also carry empty values.
	yamlKus, err := os.ReadFile("./testdata/kustomization_varsub_empty.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	clientObjects, err := readYamlObjects(strings.NewReader(string(yamlKus)))
	g.Expect(err).NotTo(HaveOccurred())

	// Ensure the namespace exists (it may already have been created by another test).
	if err := createObjectFile(kubeClient, "./testdata/ns.yaml"); err != nil {
		g.Expect(err.Error()).To(ContainSubstring("already exists"))
	}

	// Create the ConfigMap and Secret holding the empty values.
	err = createObjectFile(kubeClient, "./testdata/configmap_empty.yaml")
	g.Expect(err).NotTo(HaveOccurred())
	err = createObjectFile(kubeClient, "./testdata/secret_empty.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	fs := filesys.MakeFsOnDisk()
	resMap, err := kustomize.Build(fs, "./testdata/varsubstrictempty/")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resMap.Resources()).To(HaveLen(1))
	res := resMap.Resources()[0]

	// Strict mode must not error: every referenced var exists, even though empty.
	outRes, err := kustomize.SubstituteVariables(context.Background(),
		kubeClient, clientObjects[0], res, kustomize.SubstituteWithStrict(true))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(outRes).NotTo(BeNil())

	// All references were resolved (no leftover expressions) and the empty
	// values render as empty/null instead of causing a strict-mode error.
	yml, err := outRes.AsYAML()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(yml)).NotTo(ContainSubstring("${"))
	g.Expect(string(yml)).To(ContainSubstring("from-inline: null"))
	g.Expect(string(yml)).To(ContainSubstring("from-configmap: null"))
	g.Expect(string(yml)).To(ContainSubstring("from-secret: null"))

	// Prove that rendering empty values in strict mode produces exactly the
	// same output as strict mode off with the variables omitted entirely.
	// A dummy non-empty var is provided only to force the substitution to run
	// (it is not referenced by the resource).
	dummyKus, err := os.ReadFile("./testdata/kustomization_varsub_dummy.yaml")
	g.Expect(err).NotTo(HaveOccurred())
	dummyObjects, err := readYamlObjects(strings.NewReader(string(dummyKus)))
	g.Expect(err).NotTo(HaveOccurred())

	resMapOmitted, err := kustomize.Build(fs, "./testdata/varsubstrictempty/")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resMapOmitted.Resources()).To(HaveLen(1))

	omittedRes, err := kustomize.SubstituteVariables(context.Background(),
		kubeClient, dummyObjects[0], resMapOmitted.Resources()[0],
		kustomize.SubstituteWithStrict(false))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(omittedRes).NotTo(BeNil())

	omittedYml, err := omittedRes.AsYAML()
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(omittedYml)).To(Equal(string(yml)))
}
