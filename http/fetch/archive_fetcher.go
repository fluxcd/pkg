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
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hashicorp/go-retryablehttp"

	"github.com/fluxcd/pkg/tar"
)

// ArchiveFetcher holds the HTTP client that reties with back off when
// the file server is offline.
type ArchiveFetcher struct {
	httpClient        *retryablehttp.Client
	maxDownloadSize   int
	maxUntarSize      int
	hostnameOverwrite string
}

// FileNotFoundError is an error type used to signal 404 HTTP status code responses.
var FileNotFoundError = errors.New("file not found")

// NewArchiveFetcher configures the retryable http client used for fetching archives.
func NewArchiveFetcher(retries, maxDownloadSize, maxUntarSize int, hostnameOverwrite string) *ArchiveFetcher {
	httpClient := retryablehttp.NewClient()
	httpClient.RetryWaitMin = 5 * time.Second
	httpClient.RetryWaitMax = 30 * time.Second
	httpClient.RetryMax = retries
	httpClient.Logger = nil

	return &ArchiveFetcher{
		httpClient:        httpClient,
		maxDownloadSize:   maxDownloadSize,
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

	f, err := os.CreateTemp("", "fetch.*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(f.Name())

	// Save temporary file, but limit download to the max download size.
	if r.maxDownloadSize > 0 {
		// Headers can lie, so instead of trusting resp.ContentLength,
		// limit the download to the max download size and error in case
		// there are still bytes left.
		// Note that discarding of remaining bytes in resp.Body is a
		// requirement for Go to effectively reuse HTTP connections.
		_, err = io.Copy(f, io.LimitReader(resp.Body, int64(r.maxDownloadSize)))
		n, _ := io.Copy(io.Discard, resp.Body)
		if n > 0 {
			return fmt.Errorf("artifact is %d bytes greater than the max download size of %d bytes", n, r.maxDownloadSize)
		}
	} else {
		_, err = io.Copy(f, resp.Body)
	}
	if err != nil {
		return fmt.Errorf("failed to copy temp contents: %w", err)
	}

	// We have just filled the file, to be able to read it from
	// the start we must go back to its beginning.
	_, err = f.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek back to beginning: %w", err)
	}

	// Ensure that the checksum of the downloaded file matches the
	// known checksum.
	if err := r.verifyChecksum(checksum, f); err != nil {
		return err
	}

	// Jump back at the beginning of the file stream again.
	_, err = f.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek back to beginning again: %w", err)
	}

	// Extracts the tar file.
	if err = tar.Untar(f, dir, tar.WithMaxUntarSize(r.maxUntarSize)); err != nil {
		return fmt.Errorf("failed to extract archive (check whether file size exceeds max download size): %w", err)
	}

	return nil
}

// verifyChecksum computes the checksum of the tarball and returns an error if the computed value
// does not match the artifact advertised checksum.
func (r *ArchiveFetcher) verifyChecksum(checksum string, reader io.Reader) error {
	hasher := sha256.New()

	// Computes reader's checksum.
	if _, err := io.Copy(hasher, reader); err != nil {
		return err
	}

	if newChecksum := fmt.Sprintf("%x", hasher.Sum(nil)); newChecksum != checksum {
		return fmt.Errorf("failed to verify archive: computed checksum '%s' doesn't match provided '%s' (check whether file size exceeds max download size)",
			newChecksum, checksum)
	}

	return nil
}
