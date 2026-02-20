/*
Copyright 2020 The Flux authors

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

package version

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/Masterminds/semver/v3"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		version string
		err     bool
	}{
		{"v1.2.3", false},
		{"v2025.07.03", false},
		{"v1.0", true},
		{"v1", true},
		{"v1.2.beta", true},
		{"v1.2-5", true},
		{"v1.2-beta5", true},
		{"\nv1.2", true},
		{"v1.2.0-x.Y.0+metadata", false},
		{"v1.2.0-x.Y.0+metadata-width-hypen", false},
		{"v1.2.3-rc1-with-hypen", false},
		{"v1.2.3.4", true},
	}

	for _, tc := range tests {
		g := NewWithT(t)
		_, err := ParseVersion(tc.version)
		if tc.err {
			g.Expect(err).To(HaveOccurred(), "version: %s", tc.version)
		} else {
			g.Expect(err).NotTo(HaveOccurred(), "version: %s", tc.version)
		}
	}
}

func TestSort(t *testing.T) {
	g := NewWithT(t)

	constraint, err := semver.NewConstraint(">= 1.2.0, < 1.3.0")
	g.Expect(err).NotTo(HaveOccurred())

	sorted := Sort(constraint, []string{
		"v1.2.0",
		"v1.2.3",
		"v1.3.0",
		"1.2.4",
		"v1.1.0",
		"something-invalid",
		"v1.2.4",
		"another-invalid",
		"1.2.4",
	})

	g.Expect(sorted).To(Equal([]string{
		// Sort is stable.
		"1.2.4",
		"v1.2.4",
		"1.2.4",
		"v1.2.3",
		"v1.2.0",
	}))
}

func TestControllerMajor(t *testing.T) {
	g := NewWithT(t)
	g.Expect(ControllerMajor("flux2")).To(Equal(2))
	g.Expect(ControllerMajor("source-controller")).To(Equal(1))
	g.Expect(ControllerMajor("kustomize-controller")).To(Equal(1))
	g.Expect(ControllerMajor("helm-controller")).To(Equal(1))
	g.Expect(ControllerMajor("notification-controller")).To(Equal(1))
	g.Expect(ControllerMajor("image-reflector-controller")).To(Equal(1))
	g.Expect(ControllerMajor("image-automation-controller")).To(Equal(1))
	g.Expect(ControllerMajor("source-watcher")).To(Equal(2))

	_, err := ControllerMajor("unknown-controller")
	g.Expect(err).To(HaveOccurred())
}

func TestControllerMinorForFluxMinor(t *testing.T) {
	tests := []struct {
		name           string
		controllerName string
		flux2Minor     int
		expected       int
		expectErr      bool
	}{
		{
			name:           "flux2 identity",
			controllerName: "flux2",
			flux2Minor:     7,
			expected:       7,
		},
		{
			name:           "flux2 higher minor",
			controllerName: "flux2",
			flux2Minor:     10,
			expected:       10,
		},
		{
			name:           "source-controller at baseline",
			controllerName: "source-controller",
			flux2Minor:     7,
			expected:       7,
		},
		{
			name:           "source-controller above baseline",
			controllerName: "source-controller",
			flux2Minor:     9,
			expected:       9,
		},
		{
			name:           "helm-controller at baseline",
			controllerName: "helm-controller",
			flux2Minor:     7,
			expected:       4,
		},
		{
			name:           "helm-controller above baseline",
			controllerName: "helm-controller",
			flux2Minor:     10,
			expected:       7,
		},
		{
			name:           "image-reflector-controller at baseline",
			controllerName: "image-reflector-controller",
			flux2Minor:     7,
			expected:       0,
		},
		{
			name:           "image-reflector-controller above baseline",
			controllerName: "image-reflector-controller",
			flux2Minor:     8,
			expected:       1,
		},
		{
			name:           "source-watcher at baseline",
			controllerName: "source-watcher",
			flux2Minor:     7,
			expected:       0,
		},
		{
			name:           "unknown controller",
			controllerName: "unknown-controller",
			flux2Minor:     7,
			expectErr:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result, err := ControllerMinorForFluxMinor(tc.controllerName, tc.flux2Minor)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(tc.expected))
			}
		})
	}
}

func TestFluxMinorForControllerMinor(t *testing.T) {
	tests := []struct {
		name            string
		controllerName  string
		controllerMinor int
		expected        int
		expectErr       bool
	}{
		{
			name:            "flux2 identity",
			controllerName:  "flux2",
			controllerMinor: 7,
			expected:        7,
		},
		{
			name:            "flux2 higher minor",
			controllerName:  "flux2",
			controllerMinor: 10,
			expected:        10,
		},
		{
			name:            "source-controller at baseline",
			controllerName:  "source-controller",
			controllerMinor: 7,
			expected:        7,
		},
		{
			name:            "source-controller above baseline",
			controllerName:  "source-controller",
			controllerMinor: 9,
			expected:        9,
		},
		{
			name:            "helm-controller at baseline",
			controllerName:  "helm-controller",
			controllerMinor: 4,
			expected:        7,
		},
		{
			name:            "helm-controller above baseline",
			controllerName:  "helm-controller",
			controllerMinor: 7,
			expected:        10,
		},
		{
			name:            "image-reflector-controller at baseline",
			controllerName:  "image-reflector-controller",
			controllerMinor: 0,
			expected:        7,
		},
		{
			name:            "image-reflector-controller above baseline",
			controllerName:  "image-reflector-controller",
			controllerMinor: 1,
			expected:        8,
		},
		{
			name:            "source-watcher at baseline",
			controllerName:  "source-watcher",
			controllerMinor: 0,
			expected:        7,
		},
		{
			name:            "unknown controller",
			controllerName:  "unknown-controller",
			controllerMinor: 7,
			expectErr:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result, err := FluxMinorForControllerMinor(tc.controllerName, tc.controllerMinor)
			if tc.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(result).To(Equal(tc.expected))
			}
		})
	}
}

func TestControllerMinorFluxMinorRoundTrip(t *testing.T) {
	g := NewWithT(t)

	for controllerName := range baselineControllerMinors {
		for flux2Minor := BaselineFlux2Minor; flux2Minor <= BaselineFlux2Minor+5; flux2Minor++ {
			controllerMinor, err := ControllerMinorForFluxMinor(controllerName, flux2Minor)
			g.Expect(err).NotTo(HaveOccurred(), "controller: %s, flux2Minor: %d", controllerName, flux2Minor)

			roundTripped, err := FluxMinorForControllerMinor(controllerName, controllerMinor)
			g.Expect(err).NotTo(HaveOccurred(), "controller: %s, controllerMinor: %d", controllerName, controllerMinor)
			g.Expect(roundTripped).To(Equal(flux2Minor), "round-trip failed for controller: %s, flux2Minor: %d", controllerName, flux2Minor)
		}
	}
}
