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
	"fmt"
	"net"
	"os"
	"time"
)

// Options contains configuration settings for the artifact storage server.
type Options struct {
	// StoragePath is the path to the directory where artifacts will be stored.
	StoragePath string `json:"storagePath"`

	// StorageAddress is the host and port the server will bind to.
	StorageAddress string `json:"storageAddress"`

	// StorageAdvAddress is the in-cluster address the server will advertise to clients.
	StorageAdvAddress string `json:"storageAdvAddress"`

	// ArtifactRetentionTTL is the duration after which stale artifacts are garbage collected.
	ArtifactRetentionTTL time.Duration `json:"artifactRetentionTTL"`

	// ArtifactRetentionRecords is the maximum number of artifacts to be kept in storage after a garbage collection.
	ArtifactRetentionRecords int `json:"artifactRetentionRecords"`

	// ArtifactDigestAlgo is the hashing algorithm used to calculate the digest of artifacts.
	ArtifactDigestAlgo string `json:"artifactDigestAlgo"`
}

// GetAdvertisedAddress returns the address the artifact server will advertise to clients.
// If StorageAdvAddress is set, it is returned as is. Otherwise, it derives the advertised address
// from StorageAddress, replacing empty or wildcard hosts with the system's hostname.
func (o *Options) GetAdvertisedAddress() (string, error) {
	if o.StorageAdvAddress != "" {
		return o.StorageAdvAddress, nil
	}
	host, port, err := net.SplitHostPort(o.StorageAddress)
	if err != nil {
		return "", fmt.Errorf("invalid storage address %q: %w", o.StorageAddress, err)
	}
	switch host {
	case "":
		host = "localhost"
	case "0.0.0.0":
		host = os.Getenv("HOSTNAME")
		if host == "" {
			hn, err := os.Hostname()
			if err != nil {
				return "", fmt.Errorf("0.0.0.0 specified in storage addr but hostname is invalid: %w", err)
			}
			host = hn
		}
	}
	return net.JoinHostPort(host, port), nil
}
