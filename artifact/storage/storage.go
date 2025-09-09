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

package storage

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluxcd/pkg/apis/meta"
	"github.com/fluxcd/pkg/lockedfile"

	"github.com/fluxcd/pkg/artifact/config"
)

// Storage manages meta.Artifact tarballs on the local filesystem.
// It provides methods for creating, verifying, and garbage collecting artifacts.
type Storage struct {
	// BasePath is the local directory path where the source artifacts are stored.
	BasePath string `json:"basePath"`

	// Hostname is the file server host name used to compose the artifacts URIs.
	Hostname string `json:"hostname"`

	// ArtifactRetentionTTL is the duration of time that artifacts will be kept
	// in storage before being garbage collected.
	ArtifactRetentionTTL time.Duration `json:"artifactRetentionTTL"`

	// ArtifactRetentionRecords is the maximum number of artifacts to be kept in
	// storage after a garbage collection.
	ArtifactRetentionRecords int `json:"artifactRetentionRecords"`
}

// New creates the storage helper using the provided configuration options.
func New(opts *config.Options) (*Storage, error) {
	if opts == nil {
		return nil, fmt.Errorf("options cannot be nil")
	}

	// Get advertised address for hostname
	hostname, err := opts.GetAdvertisedAddress()
	if err != nil {
		return nil, fmt.Errorf("failed to get advertised address: %w", err)
	}

	if f, err := os.Stat(opts.StoragePath); os.IsNotExist(err) || !f.IsDir() {
		return nil, fmt.Errorf("invalid dir path: %s", opts.StoragePath)
	}

	return &Storage{
		BasePath:                 opts.StoragePath,
		Hostname:                 hostname,
		ArtifactRetentionTTL:     opts.ArtifactRetentionTTL,
		ArtifactRetentionRecords: opts.ArtifactRetentionRecords,
	}, nil
}

// NewArtifactFor returns a new meta.Artifact.
func (s Storage) NewArtifactFor(kind string, metadata metav1.Object, revision, fileName string) meta.Artifact {
	artifactPath := ArtifactPath(kind, metadata.GetNamespace(), metadata.GetName(), fileName)
	artifact := meta.Artifact{
		Path:     artifactPath,
		Revision: revision,
	}
	s.SetArtifactURL(&artifact)
	return artifact
}

// SetArtifactURL sets the URL on the given meta.Artifact.
func (s Storage) SetArtifactURL(artifact *meta.Artifact) {
	if artifact.Path == "" {
		return
	}
	format := "http://%s/%s"
	if strings.HasPrefix(s.Hostname, "http://") || strings.HasPrefix(s.Hostname, "https://") {
		format = "%s/%s"
	}
	artifact.URL = fmt.Sprintf(format, s.Hostname, strings.TrimLeft(artifact.Path, "/"))
}

// SetHostname sets the hostname of the given URL string to the current Storage.Hostname and returns the result.
func (s Storage) SetHostname(URL string) string {
	u, err := url.Parse(URL)
	if err != nil {
		return ""
	}
	u.Host = s.Hostname
	return u.String()
}

// ArtifactExist returns a boolean indicating whether the meta.Artifact exists in storage and is a regular file.
func (s Storage) ArtifactExist(artifact meta.Artifact) bool {
	fi, err := os.Lstat(s.LocalPath(artifact))
	if err != nil {
		return false
	}
	return fi.Mode().IsRegular()
}

// VerifyArtifact verifies if the Digest of the meta.Artifact matches the digest
// of the file in Storage. It returns an error if the digests don't match, or
// if it can't be verified.
func (s Storage) VerifyArtifact(artifact meta.Artifact) error {
	if artifact.Digest == "" {
		return fmt.Errorf("artifact has no digest")
	}

	d, err := digest.Parse(artifact.Digest)
	if err != nil {
		return fmt.Errorf("failed to parse artifact digest '%s': %w", artifact.Digest, err)
	}

	f, err := os.Open(s.LocalPath(artifact))
	if err != nil {
		return err
	}
	defer f.Close()

	verifier := d.Verifier()
	if _, err = io.Copy(verifier, f); err != nil {
		return err
	}
	if !verifier.Verified() {
		return fmt.Errorf("computed digest doesn't match '%s'", d.String())
	}
	return nil
}

// Lock creates a file lock for the given meta.Artifact.
func (s Storage) Lock(artifact meta.Artifact) (unlock func(), err error) {
	lockFile := s.LocalPath(artifact) + ".lock"
	mutex := lockedfile.MutexAt(lockFile)
	return mutex.Lock()
}
