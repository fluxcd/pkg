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
	"os"
	"strings"
	"testing"

	"github.com/fluxcd/pkg/kustomize"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

const resourcePath = "./testdata/resources/"

func TestKustomizationGenerator(t *testing.T) {
	g := NewWithT(t)

	// Create a kustomization file with varsub
	yamlKus, err := os.ReadFile("./testdata/kustomization.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	clientObjects, err := readYamlObjects(strings.NewReader(string(yamlKus)))
	g.Expect(err).NotTo(HaveOccurred())

	//Get a generator
	gen := kustomize.NewGenerator(clientObjects[0])
	action, err := gen.WriteFile(resourcePath, kustomize.WithSaveOriginalKustomization())
	g.Expect(err).NotTo(HaveOccurred())
	defer kustomize.CleanDirectory(resourcePath, action)

	// Get resource from directory
	fs := filesys.MakeFsOnDisk()
	resMap, err := kustomize.BuildKustomization(fs, resourcePath)
	g.Expect(err).NotTo(HaveOccurred())

	// Check that the resource has been substituted
	resources, err := resMap.AsYaml()
	g.Expect(err).NotTo(HaveOccurred())

	//load expected result
	expected, err := os.ReadFile("./testdata/kustomization_expected.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	g.Expect(string(resources)).To(Equal(string(expected)))
}
