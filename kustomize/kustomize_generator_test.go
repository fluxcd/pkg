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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/otiai10/copy"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/resmap"
	kustypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	"github.com/fluxcd/pkg/kustomize"
)

const resourcePath = "./testdata/resources/"

func TestGenerator_EmptyDir(t *testing.T) {
	g := NewWithT(t)
	dataKS, err := os.ReadFile("./testdata/empty/ks.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	ks, err := readYamlObjects(strings.NewReader(string(dataKS)))
	g.Expect(err).NotTo(HaveOccurred())

	emptyDir, err := testTempDir(t)
	g.Expect(err).NotTo(HaveOccurred())

	_, err = kustomize.NewGenerator("", ks[0]).WriteFile(emptyDir)
	g.Expect(err).NotTo(HaveOccurred())

	data, err := os.ReadFile(filepath.Join(emptyDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(data)).To(ContainSubstring("_placeholder"))

	resMap, err := kustomize.SecureBuild(emptyDir, emptyDir, false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resMap.Resources()).To(HaveLen(0))
}

func TestGenerator_NoResources(t *testing.T) {
	g := NewWithT(t)
	dataKS, err := os.ReadFile("./testdata/noResources/ks.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	ks, err := readYamlObjects(strings.NewReader(string(dataKS)))
	g.Expect(err).NotTo(HaveOccurred())

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(copy.Copy("testdata/noResources", tmpDir)).To(Succeed())
	_, err = kustomize.NewGenerator(tmpDir, ks[0]).WriteFile(tmpDir)
	g.Expect(err).NotTo(HaveOccurred())

	resMap, err := kustomize.SecureBuild(tmpDir, tmpDir, false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resMap.Resources()).To(HaveLen(0))

	data, err := os.ReadFile(filepath.Join(tmpDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(data)).To(ContainSubstring("originAnnotations"))
}

func TestGenerator_NameTransformer(t *testing.T) {
	g := NewWithT(t)
	dataKS, err := os.ReadFile("./testdata/name/ks.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	ks, err := readYamlObjects(strings.NewReader(string(dataKS)))
	g.Expect(err).NotTo(HaveOccurred())

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(copy.Copy("testdata/name", tmpDir)).To(Succeed())
	_, err = kustomize.NewGenerator(tmpDir, ks[0]).WriteFile(tmpDir)
	g.Expect(err).NotTo(HaveOccurred())

	resMap, err := kustomize.SecureBuild(tmpDir, tmpDir, false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resMap.Resources()).To(HaveLen(1))
	g.Expect(resMap.Resources()[0].GetName()).To(ContainSubstring("prefix-test-configmap-suffix"))
	g.Expect(resMap.Resources()[0].GetNamespace()).To(Equal("test-namespace"))
}

func TestKustomizationGenerator(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		secured bool
	}{
		{
			name:    "secured with securefs",
			secured: true,
		},
		{
			name:    "not secured",
			secured: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			// Create a kustomization file with varsub
			yamlKus, err := os.ReadFile("./testdata/kustomization.yaml")
			g.Expect(err).NotTo(HaveOccurred())

			clientObjects, err := readYamlObjects(strings.NewReader(string(yamlKus)))
			g.Expect(err).NotTo(HaveOccurred())

			var (
				tmpDir string
				resMap resmap.ResMap
			)
			if tt.secured {
				tmpDir = t.TempDir()
				g.Expect(copy.Copy(resourcePath, tmpDir)).To(Succeed())
				//Get a generator
				gen := kustomize.NewGenerator(tmpDir, clientObjects[0])
				action, err := gen.WriteFile(tmpDir, kustomize.WithSaveOriginalKustomization())
				g.Expect(err).NotTo(HaveOccurred())
				defer kustomize.CleanDirectory(tmpDir, action)

				// Get resource from directory
				resMap, err = kustomize.SecureBuild(tmpDir, tmpDir, false)
				g.Expect(err).NotTo(HaveOccurred())
			} else {
				//Get a generator
				gen := kustomize.NewGenerator(tmpDir, clientObjects[0])
				action, err := gen.WriteFile(resourcePath, kustomize.WithSaveOriginalKustomization())
				g.Expect(err).NotTo(HaveOccurred())
				defer kustomize.CleanDirectory(resourcePath, action)

				// Get resource from directory
				fs := filesys.MakeFsOnDisk()
				resMap, err = kustomize.Build(fs, resourcePath)
				g.Expect(err).NotTo(HaveOccurred())
			}

			// Check that the resource has been substituted
			resources, err := resMap.AsYaml()
			g.Expect(err).NotTo(HaveOccurred())

			//load expected result
			expected, err := os.ReadFile("./testdata/kustomization_expected.yaml")
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(string(resources)).To(Equal(string(expected)))
		})
	}
}

func Test_SecureBuild_panic(t *testing.T) {
	t.Run("build panic", func(t *testing.T) {
		g := NewWithT(t)

		_, err := kustomize.SecureBuild("testdata/panic", "testdata/panic", false)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("recovered from kustomize build panic"))
		// Run again to ensure the lock is released
		_, err = kustomize.SecureBuild("testdata/panic", "testdata/panic", false)
		g.Expect(err).To(HaveOccurred())
	})
}

func Test_SecureBuild_rel_basedir(t *testing.T) {
	g := NewWithT(t)

	_, err := kustomize.SecureBuild("testdata/relbase", "testdata/relbase/clusters/staging/flux-system", false)
	g.Expect(err).ToNot(HaveOccurred())
}

func Test_Components(t *testing.T) {
	tests := []struct {
		name               string
		dir                string
		fluxComponents     []string
		expectedComponents []any
	}{
		{
			name:               "test kustomization.yaml with components and Flux Kustomization without components",
			dir:                "components",
			fluxComponents:     []string{},
			expectedComponents: []any{"componentA"},
		},
		{
			name:               "test kustomization.yaml without components and Flux Kustomization with components",
			dir:                "",
			fluxComponents:     []string{"componentB", "componentC"},
			expectedComponents: []any{"componentB", "componentC"},
		},
		{
			name:               "test kustomization.yaml with components and Flux Kustomization with components",
			dir:                "components",
			fluxComponents:     []string{"componentB", "componentC"},
			expectedComponents: []any{"componentA", "componentB", "componentC"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			tmpDir, err := testTempDir(t)
			g.Expect(err).ToNot(HaveOccurred())
			if tt.dir != "" {
				g.Expect(copy.Copy(filepath.Join("./testdata", tt.dir), tmpDir)).To(Succeed())
			}
			ks := unstructured.Unstructured{Object: map[string]any{}}
			err = unstructured.SetNestedStringSlice(ks.Object, tt.fluxComponents, "spec", "components")
			g.Expect(err).ToNot(HaveOccurred())

			_, err = kustomize.NewGenerator(tmpDir, ks).WriteFile(tmpDir)
			g.Expect(err).ToNot(HaveOccurred())

			kfileYAML, err := os.ReadFile(filepath.Join(tmpDir, "kustomization.yaml"))
			g.Expect(err).ToNot(HaveOccurred())
			var k any
			g.Expect(yaml.Unmarshal(kfileYAML, &k)).To(Succeed())

			g.Expect(k.(map[string]any)["components"]).Should(Equal(tt.expectedComponents))
		})
	}
}

func Test_IsLocalRelativePath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{
			path:     "foobar.yaml",
			expected: true,
		},
		{
			path:     "./foobar.yaml",
			expected: true,
		},
		{
			path:     "file://foobar.yaml",
			expected: true,
		},
		{
			path:     "file:///foobar.yaml",
			expected: false,
		},
		{
			path:     "/foobar.yaml",
			expected: false,
		},
		{
			path:     "https://github.com/owner/repo",
			expected: false,
		},
		{
			path:     "git@github.com:owner/repo",
			expected: false,
		},
		{
			path:     "ssh://git@github.com/owner/repo",
			expected: false,
		},
		{
			path:     "github.com/owner/repo",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(kustomize.IsLocalRelativePath(tt.path)).Should(Equal(tt.expected))
		})
	}
}

func Test_IgnoreMissingComponents(t *testing.T) {
	tests := []struct {
		name                    string
		components              []string
		ignoreMissingComponents bool
		expectError             bool
		expectedComponents      []string
	}{
		{
			name:                    "missing components with ignore enabled",
			components:              []string{"existing-component", "missing-component"},
			ignoreMissingComponents: true,
			expectError:             false,
			expectedComponents:      []string{"existing-component"},
		},
		{
			name:                    "all components missing with ignore enabled",
			components:              []string{"missing-component-1", "missing-component-2"},
			ignoreMissingComponents: true,
			expectError:             false,
			expectedComponents:      nil,
		},
		{
			name:                    "all components exist",
			components:              []string{"existing-component"},
			ignoreMissingComponents: true,
			expectError:             false,
			expectedComponents:      []string{"existing-component"},
		},
		{
			name:                    "missing components with ignore disabled - should fail during build",
			components:              []string{"existing-component", "missing-component"},
			ignoreMissingComponents: false,
			expectError:             true,
			expectedComponents:      []string{"existing-component", "missing-component"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			baseDir, err := testTempDir(t)
			g.Expect(err).ToNot(HaveOccurred())

			// Create an existing component directory in the base directory
			existingComponentDir := filepath.Join(baseDir, "existing-component")
			g.Expect(os.MkdirAll(existingComponentDir, 0755)).To(Succeed())
			componentYaml := `apiVersion: kustomize.config.k8s.io/v1alpha1
kind: Component
resources: []
`
			g.Expect(os.WriteFile(filepath.Join(existingComponentDir, "kustomization.yaml"), []byte(componentYaml), 0644)).To(Succeed())

			// Create overlay directory where kustomization.yaml will be generated
			overlayDir := filepath.Join(baseDir, "overlay")
			g.Expect(os.MkdirAll(overlayDir, 0755)).To(Succeed())

			// Update component paths to be relative from overlay directory
			var overlayComponents []string
			for _, comp := range tt.components {
				overlayComponents = append(overlayComponents, "../"+comp)
			}

			// Update expected components to match the relative paths
			var expectedOverlayComponents []any
			if tt.expectedComponents != nil {
				for _, comp := range tt.expectedComponents {
					expectedOverlayComponents = append(expectedOverlayComponents, "../"+comp)
				}
			}

			// Create Flux Kustomization object with relative paths
			ks := unstructured.Unstructured{Object: map[string]any{}}
			err = unstructured.SetNestedStringSlice(ks.Object, overlayComponents, "spec", "components")
			g.Expect(err).ToNot(HaveOccurred())
			err = unstructured.SetNestedField(ks.Object, tt.ignoreMissingComponents, "spec", "ignoreMissingComponents")
			g.Expect(err).ToNot(HaveOccurred())

			// Generate kustomization in the overlay directory
			_, err = kustomize.NewGenerator(overlayDir, ks).WriteFile(overlayDir)
			g.Expect(err).ToNot(HaveOccurred())

			// Read generated kustomization.yaml
			kfileYAML, err := os.ReadFile(filepath.Join(overlayDir, "kustomization.yaml"))
			g.Expect(err).ToNot(HaveOccurred())
			var k any
			g.Expect(yaml.Unmarshal(kfileYAML, &k)).To(Succeed())

			// Check components field against the expected overlay components
			components := k.(map[string]any)["components"]
			if expectedOverlayComponents == nil {
				g.Expect(components).To(BeNil())
			} else {
				g.Expect(components).To(Equal(expectedOverlayComponents))
			}

			// Build the kustomization
			_, buildErr := kustomize.SecureBuild(baseDir, overlayDir, false)
			if tt.expectError {
				g.Expect(buildErr).To(HaveOccurred())
				g.Expect(buildErr.Error()).To(ContainSubstring("missing-component"))
			} else {
				g.Expect(buildErr).ToNot(HaveOccurred())
			}
		})
	}
}

func testTempDir(t *testing.T) (string, error) {
	tmpDir := t.TempDir()

	tmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		return "", fmt.Errorf("error evaluating symlink: '%w'", err)
	}

	return tmpDir, err
}

func TestKustomizationGenerator_WithSourceIgnore(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		ignore string
	}{
		{
			name: "without ignore",
		},
		{
			name:   "with ignore",
			ignore: "!config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			// Create a kustomization file with varsub
			yamlKus, err := os.ReadFile("./testdata/kustomization.yaml")
			g.Expect(err).NotTo(HaveOccurred())

			clientObjects, err := readYamlObjects(strings.NewReader(string(yamlKus)))
			g.Expect(err).NotTo(HaveOccurred())

			var resMap resmap.ResMap
			//Get a generator
			gen := kustomize.NewGeneratorWithIgnore("", tt.ignore, clientObjects[0])
			action, err := gen.WriteFile(resourcePath, kustomize.WithSaveOriginalKustomization())
			g.Expect(err).NotTo(HaveOccurred())
			defer kustomize.CleanDirectory(resourcePath, action)

			// Get resource from directory
			fs := filesys.MakeFsOnDisk()
			resMap, err = kustomize.Build(fs, resourcePath)
			g.Expect(err).NotTo(HaveOccurred())

			// Check that the resource has been substituted
			resources, err := resMap.AsYaml()
			g.Expect(err).NotTo(HaveOccurred())

			//load expected result
			var expected []byte
			if tt.ignore == "" {
				expected = []byte("")
			} else {
				expected, err = os.ReadFile("./testdata/kustomization_with_ignore_expected.yaml")
				g.Expect(err).NotTo(HaveOccurred())
			}

			g.Expect(string(resources)).To(Equal(string(expected)))
		})
	}
}

func TestKustomizationGenerator_WithRemoteResource(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		ignore   string
		expected []string
	}{
		{
			name: "without ignore",
			expected: []string{
				"configmap.yaml",
				"https://raw.githubusercontent.com/fluxcd/flux2/main/manifests/rbac/controller.yaml",
			},
		},
		{
			name:   "with ignore",
			ignore: "configmap.yaml",
			expected: []string{
				"https://raw.githubusercontent.com/fluxcd/flux2/main/manifests/rbac/controller.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			// Create a kustomization file with varsub
			yamlKus, err := os.ReadFile("./testdata/kustomization.yaml")
			g.Expect(err).NotTo(HaveOccurred())

			clientObjects, err := readYamlObjects(strings.NewReader(string(yamlKus)))
			g.Expect(err).NotTo(HaveOccurred())

			// Get a generator
			gen := kustomize.NewGeneratorWithIgnore("./testdata/remote", tt.ignore, clientObjects[0])
			action, err := gen.WriteFile("./testdata/remote", kustomize.WithSaveOriginalKustomization())
			g.Expect(err).NotTo(HaveOccurred())
			defer kustomize.CleanDirectory("./testdata/remote", action)

			// Read updated Kustomization contents
			updatedContent, err := os.ReadFile("./testdata/remote/kustomization.yaml")
			g.Expect(err).NotTo(HaveOccurred())

			kus := kustypes.Kustomization{
				TypeMeta: kustypes.TypeMeta{
					APIVersion: kustypes.KustomizationVersion,
					Kind:       kustypes.KustomizationKind,
				},
			}
			err = yaml.Unmarshal(updatedContent, &kus)
			g.Expect(err).NotTo(HaveOccurred())

			// Check that the resources are passed through
			g.Expect(kus.Resources).To(Equal(tt.expected))
		})
	}
}
