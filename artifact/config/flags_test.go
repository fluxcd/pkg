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
	"time"

	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"

	"github.com/fluxcd/pkg/artifact/config"
)

func Test_Options_BindFlags(t *testing.T) {
	tests := []struct {
		name                             string
		commandLine                      []string
		expectedStoragePath              string
		expectedStorageAddress           string
		expectedStorageAdvAddress        string
		expectedArtifactRetentionTTL     time.Duration
		expectedArtifactRetentionRecords int
		expectedArtifactDigestAlgo       string
	}{
		{
			name:                             "empty flags gets default values",
			commandLine:                      []string{""},
			expectedStoragePath:              "/data",
			expectedStorageAddress:           ":9090",
			expectedStorageAdvAddress:        "",
			expectedArtifactRetentionTTL:     time.Minute,
			expectedArtifactRetentionRecords: 2,
			expectedArtifactDigestAlgo:       "sha256",
		},
		{
			name:                             "storage path only",
			commandLine:                      []string{"--storage-path=/tmp/artifacts"},
			expectedStoragePath:              "/tmp/artifacts",
			expectedStorageAddress:           ":9090",
			expectedStorageAdvAddress:        "",
			expectedArtifactRetentionTTL:     time.Minute,
			expectedArtifactRetentionRecords: 2,
			expectedArtifactDigestAlgo:       "sha256",
		},
		{
			name:                             "storage address only",
			commandLine:                      []string{"--storage-addr=:8080"},
			expectedStoragePath:              "/data",
			expectedStorageAddress:           ":8080",
			expectedStorageAdvAddress:        "",
			expectedArtifactRetentionTTL:     time.Minute,
			expectedArtifactRetentionRecords: 2,
			expectedArtifactDigestAlgo:       "sha256",
		},
		{
			name:                             "storage advertise address only",
			commandLine:                      []string{"--storage-adv-addr=artifacts.example.com:9090"},
			expectedStoragePath:              "/data",
			expectedStorageAddress:           ":9090",
			expectedStorageAdvAddress:        "artifacts.example.com:9090",
			expectedArtifactRetentionTTL:     time.Minute,
			expectedArtifactRetentionRecords: 2,
			expectedArtifactDigestAlgo:       "sha256",
		},
		{
			name:                             "artifact retention TTL only",
			commandLine:                      []string{"--artifact-retention-ttl=5m"},
			expectedStoragePath:              "/data",
			expectedStorageAddress:           ":9090",
			expectedStorageAdvAddress:        "",
			expectedArtifactRetentionTTL:     5 * time.Minute,
			expectedArtifactRetentionRecords: 2,
			expectedArtifactDigestAlgo:       "sha256",
		},
		{
			name:                             "artifact retention records only",
			commandLine:                      []string{"--artifact-retention-records=10"},
			expectedStoragePath:              "/data",
			expectedStorageAddress:           ":9090",
			expectedStorageAdvAddress:        "",
			expectedArtifactRetentionTTL:     time.Minute,
			expectedArtifactRetentionRecords: 10,
			expectedArtifactDigestAlgo:       "sha256",
		},
		{
			name:                             "artifact digest algorithm only",
			commandLine:                      []string{"--artifact-digest-algo=sha512"},
			expectedStoragePath:              "/data",
			expectedStorageAddress:           ":9090",
			expectedStorageAdvAddress:        "",
			expectedArtifactRetentionTTL:     time.Minute,
			expectedArtifactRetentionRecords: 2,
			expectedArtifactDigestAlgo:       "sha512",
		},
		{
			name: "all flags set",
			commandLine: []string{
				"--storage-path=/var/artifacts",
				"--storage-addr=:9000",
				"--storage-adv-addr=artifacts.cluster.local:9000",
				"--artifact-retention-ttl=1h",
				"--artifact-retention-records=5",
				"--artifact-digest-algo=sha512",
			},
			expectedStoragePath:              "/var/artifacts",
			expectedStorageAddress:           ":9000",
			expectedStorageAdvAddress:        "artifacts.cluster.local:9000",
			expectedArtifactRetentionTTL:     time.Hour,
			expectedArtifactRetentionRecords: 5,
			expectedArtifactDigestAlgo:       "sha512",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			f := pflag.NewFlagSet("test", pflag.ContinueOnError)
			opts := config.Options{}
			opts.BindFlags(f)

			err := f.Parse(tt.commandLine)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(opts.StoragePath).To(Equal(tt.expectedStoragePath))
			g.Expect(opts.StorageAddress).To(Equal(tt.expectedStorageAddress))
			g.Expect(opts.StorageAdvAddress).To(Equal(tt.expectedStorageAdvAddress))
			g.Expect(opts.ArtifactRetentionTTL).To(Equal(tt.expectedArtifactRetentionTTL))
			g.Expect(opts.ArtifactRetentionRecords).To(Equal(tt.expectedArtifactRetentionRecords))
			g.Expect(opts.ArtifactDigestAlgo).To(Equal(tt.expectedArtifactDigestAlgo))
		})
	}
}

func Test_Options_BindFlags_WithEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name                             string
		envVars                          map[string]string
		commandLine                      []string
		expectedStoragePath              string
		expectedStorageAddress           string
		expectedStorageAdvAddress        string
		expectedArtifactRetentionTTL     time.Duration
		expectedArtifactRetentionRecords int
		expectedArtifactDigestAlgo       string
	}{
		{
			name: "environment variables override defaults",
			envVars: map[string]string{
				"STORAGE_PATH":     "/env/artifacts",
				"STORAGE_ADDRESS":  ":8080",
				"STORAGE_ADV_ADDR": "env.example.com:8080",
			},
			commandLine:                      []string{""},
			expectedStoragePath:              "/env/artifacts",
			expectedStorageAddress:           ":8080",
			expectedStorageAdvAddress:        "env.example.com:8080",
			expectedArtifactRetentionTTL:     time.Minute,
			expectedArtifactRetentionRecords: 2,
			expectedArtifactDigestAlgo:       "sha256",
		},
		{
			name: "command line flags override environment variables",
			envVars: map[string]string{
				"STORAGE_PATH":     "/env/artifacts",
				"STORAGE_ADDRESS":  ":8080",
				"STORAGE_ADV_ADDR": "env.example.com:8080",
			},
			commandLine: []string{
				"--storage-path=/flag/artifacts",
				"--storage-addr=:7070",
			},
			expectedStoragePath:              "/flag/artifacts",
			expectedStorageAddress:           ":7070",
			expectedStorageAdvAddress:        "env.example.com:8080",
			expectedArtifactRetentionTTL:     time.Minute,
			expectedArtifactRetentionRecords: 2,
			expectedArtifactDigestAlgo:       "sha256",
		},
		{
			name: "partial environment variables with defaults",
			envVars: map[string]string{
				"STORAGE_PATH": "/partial/env",
			},
			commandLine:                      []string{""},
			expectedStoragePath:              "/partial/env",
			expectedStorageAddress:           ":9090",
			expectedStorageAdvAddress:        "",
			expectedArtifactRetentionTTL:     time.Minute,
			expectedArtifactRetentionRecords: 2,
			expectedArtifactDigestAlgo:       "sha256",
		},
		{
			name: "empty environment variables use defaults",
			envVars: map[string]string{
				"STORAGE_PATH":     "",
				"STORAGE_ADDRESS":  "",
				"STORAGE_ADV_ADDR": "",
			},
			commandLine:                      []string{""},
			expectedStoragePath:              "/data",
			expectedStorageAddress:           ":9090",
			expectedStorageAdvAddress:        "",
			expectedArtifactRetentionTTL:     time.Minute,
			expectedArtifactRetentionRecords: 2,
			expectedArtifactDigestAlgo:       "sha256",
		},
		{
			name: "mixed environment and flags",
			envVars: map[string]string{
				"STORAGE_PATH":     "/env/mixed",
				"STORAGE_ADV_ADDR": "mixed.example.com:9090",
			},
			commandLine: []string{
				"--storage-addr=:6060",
				"--artifact-retention-ttl=30m",
				"--artifact-retention-records=15",
			},
			expectedStoragePath:              "/env/mixed",
			expectedStorageAddress:           ":6060",
			expectedStorageAdvAddress:        "mixed.example.com:9090",
			expectedArtifactRetentionTTL:     30 * time.Minute,
			expectedArtifactRetentionRecords: 15,
			expectedArtifactDigestAlgo:       "sha256",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Set up environment variables
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			f := pflag.NewFlagSet("test", pflag.ContinueOnError)
			opts := config.Options{}
			opts.BindFlags(f)

			err := f.Parse(tt.commandLine)
			g.Expect(err).NotTo(HaveOccurred())

			g.Expect(opts.StoragePath).To(Equal(tt.expectedStoragePath))
			g.Expect(opts.StorageAddress).To(Equal(tt.expectedStorageAddress))
			g.Expect(opts.StorageAdvAddress).To(Equal(tt.expectedStorageAdvAddress))
			g.Expect(opts.ArtifactRetentionTTL).To(Equal(tt.expectedArtifactRetentionTTL))
			g.Expect(opts.ArtifactRetentionRecords).To(Equal(tt.expectedArtifactRetentionRecords))
			g.Expect(opts.ArtifactDigestAlgo).To(Equal(tt.expectedArtifactDigestAlgo))
		})
	}
}
