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

package features

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
)

func TestSupportedFeatures(t *testing.T) {
	tests := []struct {
		name              string
		supportedFeatures map[string]bool
		commandLine       []string
		wantErr           string
	}{
		{
			name:              "opt-in when default value is false",
			commandLine:       []string{"--feature-gates=invisible-messages=true"},
			supportedFeatures: map[string]bool{"invisible-messages": false},
		},
		{
			name:              "opt-out when default value is true",
			commandLine:       []string{"--feature-gates=invisible-messages=false"},
			supportedFeatures: map[string]bool{"invisible-messages": true},
		},
		{
			name:              "multiple feature gates",
			commandLine:       []string{"--feature-gates=invisible-messages=false,time-travel=true"},
			supportedFeatures: map[string]bool{"invisible-messages": true, "time-travel": false},
		},
		{
			name:              "try set feature gate that is not supported",
			commandLine:       []string{"--feature-gates=time-travel=true"},
			supportedFeatures: map[string]bool{},
			wantErr:           "feature-gate 'time-travel' not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			fs := pflag.NewFlagSet("", pflag.ContinueOnError)

			features := FeatureGates{}
			features.BindFlags(fs)
			fs.Parse(tt.commandLine)

			err := features.SupportedFeatures(tt.supportedFeatures)
			if tt.wantErr == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).Should(ContainSubstring(tt.wantErr))
			}
		})
	}
}

func TestEnabled(t *testing.T) {
	tests := []struct {
		name                 string
		supportedFeatures    map[string]bool
		setSupportedFeatures bool
		commandLine          []string
		featureGate          string
		enabled              bool
		wantErr              string
	}{
		{
			name:                 "opt-in when default value is false",
			commandLine:          []string{"--feature-gates=invisible-messages=true"},
			featureGate:          "invisible-messages",
			supportedFeatures:    map[string]bool{"invisible-messages": false},
			setSupportedFeatures: true,
			enabled:              true,
		},
		{
			name:                 "opt-out when default value is true",
			commandLine:          []string{"--feature-gates=invisible-messages=false"},
			featureGate:          "invisible-messages",
			supportedFeatures:    map[string]bool{"invisible-messages": true},
			setSupportedFeatures: true,
			enabled:              false,
		},
		{
			name:                 "multiple feature gates",
			commandLine:          []string{"--feature-gates=invisible-messages=false,time-travel=true"},
			featureGate:          "time-travel",
			supportedFeatures:    map[string]bool{"invisible-messages": true, "time-travel": false},
			setSupportedFeatures: true,
			enabled:              true,
		},
		{
			name:                 "try feature gate that is not supported",
			featureGate:          "time-travel",
			supportedFeatures:    map[string]bool{},
			setSupportedFeatures: true,
			wantErr:              "feature-gate 'time-travel' not supported",
		},
		{
			name:                 "supported features not set",
			featureGate:          "time-travel",
			setSupportedFeatures: false,
			wantErr:              "supported features not set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			loaded = false
			fs := pflag.NewFlagSet("", pflag.ContinueOnError)

			features := FeatureGates{}
			features.BindFlags(fs)
			fs.Parse(tt.commandLine)

			if tt.setSupportedFeatures {
				err := features.SupportedFeatures(tt.supportedFeatures)
				g.Expect(err).ToNot(HaveOccurred())
			}

			enabled, err := Enabled(tt.featureGate)
			if tt.wantErr == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).Should(ContainSubstring(tt.wantErr))
			}

			g.Expect(enabled).To(Equal(tt.enabled))
		})
	}
}
