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

package config_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/artifact/config"
)

func Test_Options_GetAdvertisedAddress(t *testing.T) {
	tests := []struct {
		name              string
		storageAddress    string
		storageAdvAddress string
		expectedAddress   string
		expectedError     bool
		setHostnameEnv    bool
		hostnameEnvValue  string
	}{
		{
			name:              "uses storage adv address when set",
			storageAddress:    ":9090",
			storageAdvAddress: "artifacts.example.com:9090",
			expectedAddress:   "artifacts.example.com:9090",
			expectedError:     false,
		},
		{
			name:              "derives from storage address with empty host",
			storageAddress:    ":9090",
			storageAdvAddress: "",
			expectedAddress:   "localhost:9090",
			expectedError:     false,
		},
		{
			name:              "derives from storage address with explicit host",
			storageAddress:    "127.0.0.1:9090",
			storageAdvAddress: "",
			expectedAddress:   "127.0.0.1:9090",
			expectedError:     false,
		},
		{
			name:              "derives from 0.0.0.0 with HOSTNAME env",
			storageAddress:    "0.0.0.0:9090",
			storageAdvAddress: "",
			setHostnameEnv:    true,
			hostnameEnvValue:  "test-host",
			expectedAddress:   "test-host:9090",
			expectedError:     false,
		},
		{
			name:              "derives from 0.0.0.0 without HOSTNAME env",
			storageAddress:    "0.0.0.0:9090",
			storageAdvAddress: "",
			setHostnameEnv:    false,
			expectedError:     false,
		},
		{
			name:              "invalid storage address",
			storageAddress:    "invalid-address",
			storageAdvAddress: "",
			expectedError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Set up environment
			if tt.setHostnameEnv {
				t.Setenv("HOSTNAME", tt.hostnameEnvValue)
			}

			opts := config.Options{
				StorageAddress:    tt.storageAddress,
				StorageAdvAddress: tt.storageAdvAddress,
			}

			address, err := opts.GetAdvertisedAddress()

			if tt.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				if tt.expectedAddress != "" {
					g.Expect(address).To(Equal(tt.expectedAddress))
				} else {
					// For the case where we use system hostname, just verify it's not empty
					g.Expect(address).NotTo(BeEmpty())
					g.Expect(address).To(ContainSubstring(":9090"))
				}
			}
		})
	}
}
