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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/otiai10/copy"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func Test_Images(t *testing.T) {
	const containerImage = "ghcr.io/example/app"
	const digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	deployment := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
spec:
  selector:
    matchLabels:
      app: app
  template:
    metadata:
      labels:
        app: app
    spec:
      containers:
        - name: app
          image: ` + containerImage + `
`

	tests := []struct {
		name           string
		existingImages []kustypes.Image
		fluxImages     []kustypes.Image
		expectedImages []kustypes.Image
		expectedImage  string
	}{
		{
			name: "existing newTag is preserved when only newName is set",
			existingImages: []kustypes.Image{
				{Name: containerImage, NewTag: "v1.2.3"},
			},
			fluxImages: []kustypes.Image{
				{Name: containerImage, NewName: "registry.example.com/app"},
			},
			expectedImages: []kustypes.Image{
				{Name: containerImage, NewName: "registry.example.com/app", NewTag: "v1.2.3"},
			},
			expectedImage: "registry.example.com/app:v1.2.3",
		},
		{
			name: "existing newName is preserved when only newTag is set",
			existingImages: []kustypes.Image{
				{Name: containerImage, NewName: "registry.example.com/app"},
			},
			fluxImages: []kustypes.Image{
				{Name: containerImage, NewTag: "v2.0.0"},
			},
			expectedImages: []kustypes.Image{
				{Name: containerImage, NewName: "registry.example.com/app", NewTag: "v2.0.0"},
			},
			expectedImage: "registry.example.com/app:v2.0.0",
		},
		{
			name: "existing digest is preserved when only newName is set",
			existingImages: []kustypes.Image{
				{Name: containerImage, Digest: digest},
			},
			fluxImages: []kustypes.Image{
				{Name: containerImage, NewName: "registry.example.com/app"},
			},
			expectedImages: []kustypes.Image{
				{Name: containerImage, NewName: "registry.example.com/app", Digest: digest},
			},
			expectedImage: "registry.example.com/app@" + digest,
		},
		{
			name: "existing fields are overridden when set",
			existingImages: []kustypes.Image{
				{Name: containerImage, NewName: "old.example.com/app", NewTag: "v1.2.3"},
			},
			fluxImages: []kustypes.Image{
				{Name: containerImage, NewName: "registry.example.com/app", NewTag: "v2.0.0"},
			},
			expectedImages: []kustypes.Image{
				{Name: containerImage, NewName: "registry.example.com/app", NewTag: "v2.0.0"},
			},
			expectedImage: "registry.example.com/app:v2.0.0",
		},
		{
			name: "image without existing entry is appended",
			fluxImages: []kustypes.Image{
				{Name: containerImage, NewName: "registry.example.com/app", NewTag: "v2.0.0"},
			},
			expectedImages: []kustypes.Image{
				{Name: containerImage, NewName: "registry.example.com/app", NewTag: "v2.0.0"},
			},
			expectedImage: "registry.example.com/app:v2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			tmpDir, err := testTempDir(t)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(os.WriteFile(filepath.Join(tmpDir, "deployment.yaml"), []byte(deployment), 0o644)).To(Succeed())
			kusYAML, err := yaml.Marshal(kustypes.Kustomization{
				TypeMeta: kustypes.TypeMeta{
					APIVersion: kustypes.KustomizationVersion,
					Kind:       kustypes.KustomizationKind,
				},
				Resources: []string{"deployment.yaml"},
				Images:    tt.existingImages,
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(os.WriteFile(filepath.Join(tmpDir, "kustomization.yaml"), kusYAML, 0o644)).To(Succeed())

			fluxImages := make([]any, 0, len(tt.fluxImages))
			for _, im := range tt.fluxImages {
				m := map[string]any{"name": im.Name}
				if im.NewName != "" {
					m["newName"] = im.NewName
				}
				if im.NewTag != "" {
					m["newTag"] = im.NewTag
				}
				if im.Digest != "" {
					m["digest"] = im.Digest
				}
				fluxImages = append(fluxImages, m)
			}
			ks := unstructured.Unstructured{Object: map[string]any{}}
			g.Expect(unstructured.SetNestedSlice(ks.Object, fluxImages, "spec", "images")).To(Succeed())

			_, err = kustomize.NewGenerator(tmpDir, ks).WriteFile(tmpDir)
			g.Expect(err).ToNot(HaveOccurred())

			kfileYAML, err := os.ReadFile(filepath.Join(tmpDir, "kustomization.yaml"))
			g.Expect(err).ToNot(HaveOccurred())
			var kus kustypes.Kustomization
			g.Expect(yaml.Unmarshal(kfileYAML, &kus)).To(Succeed())
			g.Expect(kus.Images).To(Equal(tt.expectedImages))

			resMap, err := kustomize.SecureBuild(tmpDir, tmpDir, false)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(resMap.Resources()).To(HaveLen(1))

			out, err := resMap.AsYaml()
			g.Expect(err).ToNot(HaveOccurred())
			var obj map[string]interface{}
			g.Expect(yaml.Unmarshal(out, &obj)).To(Succeed())
			containers, found, err := unstructured.NestedSlice(obj, "spec", "template", "spec", "containers")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(containers).To(HaveLen(1))
			image, found, err := unstructured.NestedString(containers[0].(map[string]interface{}), "image")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(image).To(Equal(tt.expectedImage))
		})
	}
}

func TestGenerator_BuildMetadata(t *testing.T) {
	g := NewWithT(t)
	dataKS, err := os.ReadFile("./testdata/buildMetadata/ks.yaml")
	g.Expect(err).NotTo(HaveOccurred())

	ks, err := readYamlObjects(strings.NewReader(string(dataKS)))
	g.Expect(err).NotTo(HaveOccurred())

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(copy.Copy("testdata/buildMetadata", tmpDir)).To(Succeed())
	_, err = kustomize.NewGenerator(tmpDir, ks[0]).WriteFile(tmpDir)
	g.Expect(err).NotTo(HaveOccurred())

	// Read the generated kustomization.yaml and verify buildMetadata is set
	kfileYAML, err := os.ReadFile(filepath.Join(tmpDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())

	var kus kustypes.Kustomization
	g.Expect(yaml.Unmarshal(kfileYAML, &kus)).To(Succeed())
	g.Expect(kus.BuildMetadata).To(Equal([]string{"originAnnotations", "transformerAnnotations"}))

	// Verify that the build succeeds with buildMetadata set
	resMap, err := kustomize.SecureBuild(tmpDir, tmpDir, false)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(resMap.Resources()).To(HaveLen(1))
}

func TestGenerator_BuildMetadata_EmptySpec(t *testing.T) {
	g := NewWithT(t)

	// Flux Kustomization with no buildMetadata should not override defaults
	ks := unstructured.Unstructured{Object: map[string]any{}}

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(copy.Copy("testdata/buildMetadata", tmpDir)).To(Succeed())
	_, err = kustomize.NewGenerator(tmpDir, ks).WriteFile(tmpDir)
	g.Expect(err).NotTo(HaveOccurred())

	kfileYAML, err := os.ReadFile(filepath.Join(tmpDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())

	var kus kustypes.Kustomization
	g.Expect(yaml.Unmarshal(kfileYAML, &kus)).To(Succeed())
	// The kustomization.yaml has no .resources field (only configMapGenerator),
	// so the fallback "originAnnotations" is set to avoid empty build errors
	g.Expect(kus.BuildMetadata).To(Equal([]string{"originAnnotations"}))
}

func TestGenerator_BuildMetadata_NoResources(t *testing.T) {
	g := NewWithT(t)

	// Flux Kustomization with buildMetadata but no resources in the kustomization.yaml
	// The spec buildMetadata should override the fallback "originAnnotations"
	ks := unstructured.Unstructured{Object: map[string]any{}}
	err := unstructured.SetNestedStringSlice(ks.Object,
		[]string{"transformerAnnotations"}, "spec", "buildMetadata")
	g.Expect(err).ToNot(HaveOccurred())

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(copy.Copy("testdata/noResources", tmpDir)).To(Succeed())
	_, err = kustomize.NewGenerator(tmpDir, ks).WriteFile(tmpDir)
	g.Expect(err).NotTo(HaveOccurred())

	kfileYAML, err := os.ReadFile(filepath.Join(tmpDir, "kustomization.yaml"))
	g.Expect(err).NotTo(HaveOccurred())

	var kus kustypes.Kustomization
	g.Expect(yaml.Unmarshal(kfileYAML, &kus)).To(Succeed())
	// User-specified buildMetadata should override the fallback
	g.Expect(kus.BuildMetadata).To(Equal([]string{"transformerAnnotations"}))
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
			_, err = kustomize.NewGenerator(baseDir, ks).WriteFile(overlayDir)
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

func TestGenerateManifest(t *testing.T) {
	tests := []struct {
		name           string
		sourceDir      string
		ksFile         string
		expectedAction kustomize.Action
		checkManifest  func(g Gomega, manifest []byte)
	}{
		{
			name:           "existing kustomization returns unchanged action and file content",
			sourceDir:      "./testdata/resources",
			ksFile:         "./testdata/kustomization.yaml",
			expectedAction: kustomize.UnchangedAction,
			checkManifest: func(g Gomega, manifest []byte) {
				var kus kustypes.Kustomization
				g.Expect(yaml.Unmarshal(manifest, &kus)).To(Succeed())
				g.Expect(kus.Resources).To(ContainElement("./deployment.yaml"))
				g.Expect(kus.Resources).To(ContainElement("./config.yaml"))
				g.Expect(kus.Namespace).To(Equal("apps"))
			},
		},
		{
			name:           "empty dir returns created action with placeholder",
			sourceDir:      "",
			ksFile:         "./testdata/empty/ks.yaml",
			expectedAction: kustomize.CreatedAction,
			checkManifest: func(g Gomega, manifest []byte) {
				g.Expect(string(manifest)).To(ContainSubstring("_placeholder"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			dataKS, err := os.ReadFile(tt.ksFile)
			g.Expect(err).NotTo(HaveOccurred())
			ks, err := readYamlObjects(strings.NewReader(string(dataKS)))
			g.Expect(err).NotTo(HaveOccurred())

			var dirPath string
			if tt.sourceDir != "" {
				tmpDir, err := testTempDir(t)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(copy.Copy(tt.sourceDir, tmpDir)).To(Succeed())
				dirPath = tmpDir
			} else {
				tmpDir, err := testTempDir(t)
				g.Expect(err).NotTo(HaveOccurred())
				dirPath = tmpDir
			}

			beforeEntries := snapshotDir(g, dirPath)

			gen := kustomize.NewGenerator(dirPath, ks[0])
			manifest, kfile, action, err := gen.GenerateManifest(dirPath)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(action).To(Equal(tt.expectedAction))
			g.Expect(manifest).NotTo(BeEmpty())

			// full path, resolvable relative to dirPath
			g.Expect(kfile).To(HavePrefix(dirPath))
			g.Expect(filepath.Base(kfile)).To(HavePrefix("kustomization"))

			tt.checkManifest(g, manifest)

			// no disk writes
			afterEntries := snapshotDir(g, dirPath)
			g.Expect(afterEntries).To(Equal(beforeEntries))
		})
	}
}

func snapshotDir(g Gomega, dir string) map[string]string {
	entries := map[string]string{}
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		entries[rel] = string(data)
		return nil
	})
	g.Expect(err).NotTo(HaveOccurred())
	return entries
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

func TestOpenAPIPollution(t *testing.T) {
	t.Run("custom OpenAPI schema (Deployment only) does not affect StatefulSet builds", func(t *testing.T) {
		g := NewWithT(t)

		tmpDir1, err := testTempDir(t)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(copy.Copy("testdata/openapiPollution/pollutor", tmpDir1)).To(Succeed())

		// Build 1: Deployment build with custom openapi schema (includes Deployment but NOT StatefulSet)
		res1, err := kustomize.SecureBuild(tmpDir1, tmpDir1, false)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(res1.Resources()).To(HaveLen(1))
		yaml1, err := res1.AsYaml()
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the Deployment patch was applied correctly (resource requests/limits added)
		g.Expect(string(yaml1)).To(ContainSubstring("image: nginx:latest"))
		g.Expect(string(yaml1)).To(ContainSubstring("resources:"))
		g.Expect(string(yaml1)).To(ContainSubstring("requests:"))
		g.Expect(string(yaml1)).To(ContainSubstring("memory: 64Mi"))

		tmpDir2, err := testTempDir(t)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(copy.Copy("testdata/openapiPollution/victim", tmpDir2)).To(Succeed())

		// Build 2: StatefulSet build without custom openapi
		// This should work correctly even though Build 1 used a Deployment-only schema
		res2, err := kustomize.SecureBuild(tmpDir2, tmpDir2, false)
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(res2.Resources()).To(HaveLen(1))
		yaml2, err := res2.AsYaml()
		g.Expect(err).ToNot(HaveOccurred())

		// Verify the StatefulSet patch was applied correctly
		// The bug would cause this to fail because the Deployment-only schema doesn't define how
		// to merge containers in a StatefulSet and "image: ..." would be missing
		g.Expect(string(yaml2)).To(ContainSubstring("image: busybox:latest"))
		g.Expect(string(yaml2)).To(ContainSubstring("resources:"))
		g.Expect(string(yaml2)).To(ContainSubstring("requests:"))
		g.Expect(string(yaml2)).To(ContainSubstring("memory: 64Mi"))
	})
}

func TestOpenAPIPathMergesBuiltins(t *testing.T) {
	g := NewWithT(t)

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	writeOpenAPIPathDaemonSetFixture(g, tmpDir, "custom-openapi.json")

	res, err := kustomize.SecureBuild(tmpDir, tmpDir, false)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Resources()).To(HaveLen(1))

	expectOpenAPIPathDaemonSet(g, res)
}

func TestOpenAPIPathHTTPURLMergesBuiltins(t *testing.T) {
	g := NewWithT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.Expect(r.URL.Path).To(Equal("/custom-openapi.json"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(customOpenAPISchema))
		g.Expect(err).ToNot(HaveOccurred())
	}))
	defer server.Close()

	tmpDir, err := testTempDir(t)
	g.Expect(err).ToNot(HaveOccurred())
	writeOpenAPIPathDaemonSetFixture(g, tmpDir, server.URL+"/custom-openapi.json")

	res, err := kustomize.SecureBuild(tmpDir, tmpDir, false)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.Resources()).To(HaveLen(1))

	expectOpenAPIPathDaemonSet(g, res)
}

func expectOpenAPIPathDaemonSet(g Gomega, res resmap.ResMap) {
	out, err := res.AsYaml()
	g.Expect(err).ToNot(HaveOccurred())
	obj := daemonSetObject(g, out)
	container := daemonSetContainer(g, obj.Object)
	image, found, err := unstructured.NestedString(container, "image")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(image).To(Equal("quay.io/prometheus/node-exporter:v1.10.2"))

	volumeMounts, foundVolumeMounts, err := unstructured.NestedSlice(container, "volumeMounts")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(foundVolumeMounts).To(BeTrue())
	g.Expect(volumeMounts).To(BeEmpty())

	g.Expect(kubeClient.Create(context.Background(), obj, client.DryRunAll)).To(Succeed())
}

func daemonSetObject(g Gomega, data []byte) *unstructured.Unstructured {
	var obj map[string]interface{}
	g.Expect(yaml.Unmarshal(data, &obj)).To(Succeed())
	u := &unstructured.Unstructured{Object: obj}
	u.SetNamespace("default")
	return u
}

func daemonSetContainer(g Gomega, obj map[string]interface{}) map[string]interface{} {
	containers, found, err := unstructured.NestedSlice(obj, "spec", "template", "spec", "containers")
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(found).To(BeTrue())
	g.Expect(containers).To(HaveLen(1))
	container, ok := containers[0].(map[string]interface{})
	g.Expect(ok).To(BeTrue())
	return container
}

func writeOpenAPIPathDaemonSetFixture(g Gomega, dir, openAPIPath string) {
	files := map[string]string{
		"kustomization.yaml": fmt.Sprintf(`resources:
- daemonset.yaml
openapi:
  path: %s
patches:
- patch: |-
    apiVersion: apps/v1
    kind: DaemonSet
    metadata:
      name: monitoring-prometheus-node-exporter
    spec:
      template:
        spec:
          containers:
          - name: node-exporter
            volumeMounts:
            - name: root
              mountPropagation: None
  target:
    group: apps
    kind: DaemonSet
    name: monitoring-prometheus-node-exporter
`, openAPIPath),
		"custom-openapi.json": customOpenAPISchema,
		"daemonset.yaml": `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: monitoring-prometheus-node-exporter
spec:
  selector:
    matchLabels:
      app: node-exporter
  template:
    metadata:
      labels:
        app: node-exporter
    spec:
      containers:
      - name: node-exporter
        image: quay.io/prometheus/node-exporter:v1.10.2
        volumeMounts:
        - name: root
          mountPath: /host/root
      volumes:
      - name: root
        hostPath:
          path: /
`,
	}

	for name, content := range files {
		g.Expect(os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)).To(Succeed())
	}
}

const customOpenAPISchema = `{
  "swagger": "2.0",
  "info": {
    "title": "Custom schema",
    "version": "v1"
  },
  "paths": {},
  "definitions": {}
}
`
