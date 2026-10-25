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
	"context"
	_ "crypto/sha256"
	_ "crypto/sha512"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/opencontainers/go-digest"
	_ "github.com/opencontainers/go-digest/blake3"

	"github.com/fluxcd/pkg/tar"
)

// ArchiveFetcher is a flexible API for downloading an archive from an HTTP server,
// verifying its digest and extracting its contents to a given path in the Filesystem.
type ArchiveFetcher struct {
	retries           int
	maxDownloadSize   int
	fileMode          fs.FileMode
	untarOpts         []tar.Option
	hostnameOverwrite string
	filename          string
	logger            any

	httpClient *retryablehttp.Client
	tlsConfig  *tls.Config
}

// Option is an option for constructing the ArchiveFetcher.
type Option func(a *ArchiveFetcher)

// ErrFileNotFound is an error type used to signal 404 HTTP status code responses.
var ErrFileNotFound = errors.New("file not found")

// ErrDigestMismatch is an error type used to signal that the downloaded content
// doesn't match the expected digest, e.g. because the artifact changed between
// reading the source status and downloading it.
var ErrDigestMismatch = errors.New("computed digest doesn't match")

// WithRetries sets the maximum amount of retries the HTTP client will be allowed to make.
func WithRetries(retries int) Option {
	return func(a *ArchiveFetcher) {
		a.retries = retries
	}
}

// WithMaxDownloadSize specifies a limit for the size of the downloaded archive.
func WithMaxDownloadSize(maxDownloadSize int) Option {
	return func(a *ArchiveFetcher) {
		a.maxDownloadSize = maxDownloadSize
	}
}

// WithUntar tells the ArchiveFetcher to untar the archive expecting it to be a tarball.
func WithUntar(opts ...tar.Option) Option {
	return func(a *ArchiveFetcher) {
		a.untarOpts = append([]tar.Option{}, opts...) // to make sure a.untarOpts won't be nil
	}
}

// WithHostnameOverwrite sets an override for the hostname in download URLs.
func WithHostnameOverwrite(hostnameOverwrite string) Option {
	return func(a *ArchiveFetcher) {
		a.hostnameOverwrite = hostnameOverwrite
	}
}

// WithLogger sets a logger for the HTTP client.
// The logger can be any type that implements the retryablehttp.Logger or
// retryablehttp.LeveledLogger interface. If the logger is of type logr.Logger,
// it will be wrapped in a retryablehttp.LeveledLogger that only logs errors.
func WithLogger(logger any) Option {
	return func(a *ArchiveFetcher) {
		a.logger = logger
	}
}

// WithFileName sets the file name for the downloaded archive.
func WithFileName(filename string) Option {
	return func(a *ArchiveFetcher) {
		a.filename = filename
	}
}

// WithFileMode sets the file mode for the downloaded archive.
// Applies only if untar is not enabled and the archive is not extracted.
func WithFileMode(fileMode fs.FileMode) Option {
	return func(a *ArchiveFetcher) {
		a.fileMode = fileMode
	}
}

// WithTLSConfig sets the TLS configuration for the HTTP client.
func WithTLSConfig(tlsConfig *tls.Config) Option {
	return func(a *ArchiveFetcher) {
		a.tlsConfig = tlsConfig
	}
}

// New creates an *ArchiveFetcher accepting options.
func New(opts ...Option) *ArchiveFetcher {
	a := &ArchiveFetcher{
		fileMode: 0o600,
	}
	for _, opt := range opts {
		opt(a)
	}

	// Create HTTP client.
	a.httpClient = retryablehttp.NewClient()
	a.httpClient.RetryWaitMin = 5 * time.Second
	a.httpClient.RetryWaitMax = 30 * time.Second
	a.httpClient.RetryMax = a.retries
	switch a.logger.(type) {
	case logr.Logger:
		a.httpClient.Logger = newErrorLogger(a.logger.(logr.Logger))
	default:
		a.httpClient.Logger = a.logger
	}
	if a.tlsConfig != nil {
		tr := a.httpClient.HTTPClient.Transport.(*http.Transport).Clone()
		tr.TLSClientConfig = a.tlsConfig
		a.httpClient.HTTPClient.Transport = tr
	}

	return a
}

// NewArchiveFetcher configures the retryable HTTP client used for fetching archives.
//
// Deprecated: Use New() instead.
func NewArchiveFetcher(retries, maxDownloadSize, maxUntarSize int, hostnameOverwrite string) *ArchiveFetcher {
	return NewArchiveFetcherWithLogger(retries, maxDownloadSize, maxUntarSize, hostnameOverwrite, nil)
}

// NewArchiveFetcherWithLogger configures the retryable HTTP client used for
// fetching archives and sets the logger to use.
//
// The logger can be any type that implements the retryablehttp.Logger or
// retryablehttp.LeveledLogger interface. If the logger is of type logr.Logger,
// it will be wrapped in a retryablehttp.LeveledLogger that only logs errors.
//
// Deprecated: Use New() instead.
func NewArchiveFetcherWithLogger(retries, maxDownloadSize, maxUntarSize int, hostnameOverwrite string, logger any) *ArchiveFetcher {
	return New(WithRetries(retries), WithMaxDownloadSize(maxDownloadSize), WithHostnameOverwrite(hostnameOverwrite),
		WithLogger(logger), WithUntar(tar.WithMaxUntarSize(maxUntarSize)))
}

// Fetch is the same as FetchWithContext but uses a background context.
//
// Deprecated: Use FetchWithContext instead.
func (r *ArchiveFetcher) Fetch(archiveURL, digest, dir string) error {
	return r.FetchWithContext(context.Background(), archiveURL, digest, dir)
}

// FetchBytes is the same as FetchWithContext but returns the content of the archive
// as a bytes.Buffer instead of writing it to a directory. Untar options are
// ignored.
func (r *ArchiveFetcher) FetchBytes(ctx context.Context, archiveURL, digest string) (bytes.Buffer, error) {
	resp, err := r.download(ctx, archiveURL)
	if err != nil {
		return bytes.Buffer{}, err
	}
	defer resp.Body.Close()

	var buf bytes.Buffer
	if err := r.copyResponse(&buf, resp); err != nil {
		return bytes.Buffer{}, err
	}
	if err := r.verifyDigest(digest, bytes.NewReader(buf.Bytes())); err != nil {
		return bytes.Buffer{}, fmt.Errorf("failed to verify archive: %w", err)
	}
	return buf, nil
}

// download performs the HTTP request and validates the response status.
func (r *ArchiveFetcher) download(ctx context.Context, archiveURL string) (*http.Response, error) {
	if r.hostnameOverwrite != "" || r.tlsConfig != nil {
		u, err := url.Parse(archiveURL)
		if err != nil {
			return nil, err
		}
		if r.hostnameOverwrite != "" {
			u.Host = r.hostnameOverwrite
		}
		if r.tlsConfig != nil {
			u.Scheme = "https"
		}
		archiveURL = u.String()
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, http.MethodGet, archiveURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create a new request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download archive: %w", err)
	}

	if code := resp.StatusCode; code != http.StatusOK {
		_ = resp.Body.Close()
		if code == http.StatusNotFound {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("failed to download archive from %s (status: %s)", archiveURL, resp.Status)
	}
	return resp, nil
}

// copyResponse copies the response body to the given writer, enforcing the
// configured maximum download size.
func (r *ArchiveFetcher) copyResponse(dst io.Writer, resp *http.Response) error {
	if r.maxDownloadSize > 0 {
		// Headers can lie, so instead of trusting resp.ContentLength,
		// limit the download to the max download size and error in case
		// there are still bytes left.
		// Note that discarding of remaining bytes in resp.Body is a
		// requirement for Go to effectively reuse HTTP connections.
		if _, err := io.Copy(dst, io.LimitReader(resp.Body, int64(r.maxDownloadSize))); err != nil {
			return fmt.Errorf("failed to copy archive contents: %w", err)
		}
		n, _ := io.Copy(io.Discard, resp.Body)
		if n > 0 {
			return fmt.Errorf("artifact is %d bytes greater than the max download size of %d bytes", n, r.maxDownloadSize)
		}
		return nil
	}

	if _, err := io.Copy(dst, resp.Body); err != nil {
		return fmt.Errorf("failed to copy archive contents: %w", err)
	}
	return nil
}

// FetchWithContext downloads, verifies and extracts the tarball content to the specified directory.
// If the file server responds with 5xx errors, the download operation is retried.
// If the file server responds with 404, the returned error is of type ErrFileNotFound.
// If the digest of the downloaded content doesn't match, the returned error wraps ErrDigestMismatch.
// If the file server is unavailable for more than 3 minutes, the returned error contains the original status code.
func (r *ArchiveFetcher) FetchWithContext(ctx context.Context, archiveURL, digest, dir string) (err error) {
	resp, err := r.download(ctx, archiveURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create a file for storing the archive.
	var f *os.File
	if r.untarOpts != nil {
		f, err = os.CreateTemp("", "fetch.*.tmp")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(f.Name())
	} else {
		fn := r.filename
		if fn == "" {
			fn = path.Base(archiveURL)
		}
		p := filepath.Join(dir, fn)
		f, err = os.OpenFile(p, os.O_RDWR|os.O_CREATE|os.O_EXCL, r.fileMode)
		if err != nil {
			return fmt.Errorf("failed to create target file: %w", err)
		}
	}

	// Close the file.
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			closeErr = fmt.Errorf("failed to close file: %w", closeErr)
			if err != nil {
				err = errors.Join(err, closeErr)
			} else {
				err = closeErr
			}
		}
	}()

	// Save temporary file, but limit download to the max download size.
	if err := r.copyResponse(f, resp); err != nil {
		return err
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

	if r.untarOpts != nil {
		// Jump back at the beginning of the file stream again.
		_, err = f.Seek(0, 0)
		if err != nil {
			return fmt.Errorf("failed to seek back to beginning again: %w", err)
		}

		// Extracts the tar file.
		opts := append(r.untarOpts, tar.WithSkipSymlinks())
		if err = tar.Untar(f, dir, opts...); err != nil {
			return fmt.Errorf("failed to extract archive (check whether file size exceeds max download size): %w", err)
		}
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
		return fmt.Errorf("%w provided '%s'", ErrDigestMismatch, dig)
	}
	return nil
}
