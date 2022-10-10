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

package fetch

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/fluxcd/pkg/tar"
)

// ArchiveFetcher holds the HTTP client that reties with back off when
// the file server is offline.
type ArchiveFetcher struct {
	httpClient        *retryablehttp.Client
	maxUntarSize      int
	hostnameOverwrite string
}

// FileNotFoundError is an error type used to signal 404 HTTP status code responses.
var FileNotFoundError = errors.New("file not found")

// NewArchiveFetcher configures the retryable http client used for fetching archives.
func NewArchiveFetcher(retries, maxUntarSize int, hostnameOverwrite string) *ArchiveFetcher {
	httpClient := retryablehttp.NewClient()
	httpClient.RetryWaitMin = 5 * time.Second
	httpClient.RetryWaitMax = 30 * time.Second
	httpClient.RetryMax = retries
	httpClient.Logger = nil

	return &ArchiveFetcher{
		httpClient:        httpClient,
		maxUntarSize:      maxUntarSize,
		hostnameOverwrite: hostnameOverwrite,
	}
}

// Fetch downloads, verifies and extracts the tarball content to the specified directory.
// If the file server responds with 5xx errors, the download operation is retried.
// If the file server responds with 404, the returned error is of type FileNotFoundError.
// If the file server is unavailable for more than 3 minutes, the returned error contains the original status code.
func (r *ArchiveFetcher) Fetch(archiveURL, checksum, dir string) error {
	if r.hostnameOverwrite != "" {
		u, err := url.Parse(archiveURL)
		if err != nil {
			return err
		}
		u.Host = r.hostnameOverwrite
		archiveURL = u.String()
	}

	req, err := retryablehttp.NewRequest(http.MethodGet, archiveURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create a new request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download archive, error: %w", err)
	}
	defer resp.Body.Close()

	if code := resp.StatusCode; code != http.StatusOK {
		if code == http.StatusNotFound {
			return FileNotFoundError
		}
		return fmt.Errorf("failed to download archive from %s, status: %s", archiveURL, resp.Status)
	}

	var buf bytes.Buffer

	// verify checksum matches origin
	if err := r.verifyChecksum(checksum, &buf, resp.Body); err != nil {
		return err
	}

	// extract
	if err = tar.Untar(&buf, dir, tar.WithMaxUntarSize(r.maxUntarSize)); err != nil {
		return fmt.Errorf("failed to extract archive, error: %w", err)
	}

	return nil
}

// verifyChecksum computes the checksum of the tarball and returns an error if the computed value
// does not match the artifact advertised checksum.
func (r *ArchiveFetcher) verifyChecksum(checksum string, buf *bytes.Buffer, reader io.Reader) error {
	hasher := sha256.New()

	// compute checksum
	mw := io.MultiWriter(hasher, buf)
	if _, err := io.Copy(mw, reader); err != nil {
		return err
	}

	if newChecksum := fmt.Sprintf("%x", hasher.Sum(nil)); newChecksum != checksum {
		return fmt.Errorf("failed to verify archive: computed checksum '%s' doesn't match provided '%s'",
			newChecksum, checksum)
	}

	return nil
}
