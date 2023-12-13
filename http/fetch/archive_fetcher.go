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
	_ "crypto/sha256"
	_ "crypto/sha512"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/opencontainers/go-digest"
	_ "github.com/opencontainers/go-digest/blake3"

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

// ErrFileNotFound is an error type used to signal 404 HTTP status code responses.
var ErrFileNotFound = errors.New("file not found")

// NewArchiveFetcher configures the retryable HTTP client used for fetching archives.
func NewArchiveFetcher(retries, maxDownloadSize, maxUntarSize int, hostnameOverwrite string) *ArchiveFetcher {
	return NewArchiveFetcherWithLogger(retries, maxDownloadSize, maxUntarSize, hostnameOverwrite, nil)
}

// NewArchiveFetcherWithLogger configures the retryable HTTP client used for
// fetching archives and sets the logger to use.
//
// The logger can be any type that implements the retryablehttp.Logger or
// retryablehttp.LeveledLogger interface. If the logger is of type logr.Logger,
// it will be wrapped in a retryablehttp.LeveledLogger that only logs errors.
func NewArchiveFetcherWithLogger(retries, maxDownloadSize, maxUntarSize int, hostnameOverwrite string, logger any) *ArchiveFetcher {
	httpClient := retryablehttp.NewClient()
	httpClient.RetryWaitMin = 5 * time.Second
	httpClient.RetryWaitMax = 30 * time.Second
	httpClient.RetryMax = retries

	switch logger.(type) {
	case logr.Logger:
		httpClient.Logger = newErrorLogger(logger.(logr.Logger))
	default:
		httpClient.Logger = logger
	}

	return &ArchiveFetcher{
		httpClient:        httpClient,
		maxDownloadSize:   maxDownloadSize,
		maxUntarSize:      maxUntarSize,
		hostnameOverwrite: hostnameOverwrite,
	}
}

// Fetch downloads, verifies and extracts the tarball content to the specified directory.
// If the file server responds with 5xx errors, the download operation is retried.
// If the file server responds with 404, the returned error is of type ErrFileNotFound.
// If the file server is unavailable for more than 3 minutes, the returned error contains the original status code.
func (r *ArchiveFetcher) Fetch(archiveURL, digest, dir string) error {
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
		return fmt.Errorf("failed to download archive: %w", err)
	}
	defer resp.Body.Close()

	if code := resp.StatusCode; code != http.StatusOK {
		if code == http.StatusNotFound {
			return ErrFileNotFound
		}
		return fmt.Errorf("failed to download archive from %s (status: %s)", archiveURL, resp.Status)
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

	// Ensure that the digest of the downloaded file matches the
	// known digest.
	if err := r.verifyDigest(digest, f); err != nil {
		return fmt.Errorf("failed to verify archive: %w", err)
	}

	// Jump back at the beginning of the file stream again.
	_, err = f.Seek(0, 0)
	if err != nil {
		return fmt.Errorf("failed to seek back to beginning again: %w", err)
	}

	// Extracts the tar file.
	if err = tar.Untar(f, dir, tar.WithMaxUntarSize(r.maxUntarSize), tar.WithSkipSymlinks()); err != nil {
		return fmt.Errorf("failed to extract archive (check whether file size exceeds max download size): %w", err)
	}

	return nil
}

// verifyDigest verifies the digest of the reader, and returns an error if it
// doesn't match, fails to parse, or is empty.
func (r *ArchiveFetcher) verifyDigest(dig string, reader io.Reader) error {
	if dig == "" {
		return fmt.Errorf("empty digest")
	}

	if !strings.Contains(dig, ":") {
		dig = "sha256:" + dig
	}

	d, err := digest.Parse(dig)
	if err != nil {
		return fmt.Errorf("failed to parse digest '%s': %w", dig, err)
	}

	// Verify reader's data.
	verifier := d.Verifier()
	if _, err := io.Copy(verifier, reader); err != nil {
		return err
	}
	if !verifier.Verified() {
		return fmt.Errorf("computed digest doesn't match provided '%s' (check whether file size exceeds max download size)", dig)
	}
	return nil
}
