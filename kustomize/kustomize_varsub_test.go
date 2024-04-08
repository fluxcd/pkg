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
