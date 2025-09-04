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

package config

import (
	"os"
	"time"

	"github.com/spf13/pflag"
)

const (
	flagStoragePath    = "storage-path"
	envStoragePath     = "STORAGE_PATH"
	defaultStoragePath = "/data"

	flagStorageAddress    = "storage-addr"
	envStorageAddress     = "STORAGE_ADDRESS"
	defaultStorageAddress = ":9090"

	flagStorageAdvAddress = "storage-adv-addr"
	envStorageAdvAddress  = "STORAGE_ADV_ADDR"

	flagArtifactRetentionTTL    = "artifact-retention-ttl"
	defaultArtifactRetentionTTL = time.Minute

	flagArtifactRetentionRecords    = "artifact-retention-records"
	defaultArtifactRetentionRecords = 2

	flagArtifactDigestAlgo    = "artifact-digest-algo"
	defaultArtifactDigestAlgo = "sha256"
)

// BindFlags will parse the given pflag.FlagSet for the controller and set the Options accordingly.
func (o *Options) BindFlags(fs *pflag.FlagSet) {
	fs.StringVar(&o.StoragePath, flagStoragePath,
		envOrDefault(envStoragePath, defaultStoragePath),
		"The path to the directory where artifacts will be stored.")

	fs.StringVar(&o.StorageAddress, flagStorageAddress,
		envOrDefault(envStorageAddress, defaultStorageAddress),
		"The address the artifact server will bind to.")

	fs.StringVar(&o.StorageAdvAddress, flagStorageAdvAddress,
		envOrDefault(envStorageAdvAddress, ""),
		"The in-cluster address the artifact server will advertise to clients.")

	fs.DurationVar(&o.ArtifactRetentionTTL, flagArtifactRetentionTTL,
		defaultArtifactRetentionTTL,
		"The duration after which stale artifacts are garbage collected.")

	fs.IntVar(&o.ArtifactRetentionRecords, flagArtifactRetentionRecords,
		defaultArtifactRetentionRecords,
		"The maximum number of artifacts to be kept in storage after a garbage collection.")

	fs.StringVar(&o.ArtifactDigestAlgo, flagArtifactDigestAlgo,
		defaultArtifactDigestAlgo,
		"The hashing algorithm used to calculate the digest of artifacts.")
}

// envOrDefault returns the value of the environment variable named by the key.
// If the variable is empty or not present, it returns the defaultValue instead.
func envOrDefault(envName, defaultValue string) string {
	ret := os.Getenv(envName)
	if ret != "" {
		return ret
	}

	return defaultValue
}
