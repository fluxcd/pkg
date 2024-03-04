/*
Copyright 2024 The Flux authors

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

package client

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/hashicorp/go-retryablehttp"
	"golang.org/x/sync/errgroup"
)

const (
	minChunkSize          = 100 * 1024 * 1024 // 100MB
	maxChunkSize          = 1 << 30           // 1GB
	defaultNumberOfChunks = 50
)

var (
	// errRangeRequestNotSupported is returned when the registry does not support range requests.
	errRangeRequestNotSupported = fmt.Errorf("range requests are not supported by the registry")
	errCopyFailed               = errors.New("copy failed")
)

var (
	retries             = 3
	defaultRetryBackoff = remote.Backoff{
		Duration: 1.0 * time.Second,
		Factor:   3.0,
		Jitter:   0.1,
		Steps:    retries,
	}
)

type downloadOption func(*downloadOptions)

type downloadOptions struct {
	transport      http.RoundTripper
	auth           authn.Authenticator
	keychain       authn.Keychain
	numberOfChunks int
}

type blobManager struct {
	name   name.Reference
	c      *retryablehttp.Client
	layer  v1.Layer
	path   string
	digest v1.Hash
	size   int64
	downloadOptions
}

func withTransport(t http.RoundTripper) downloadOption {
	return func(o *downloadOptions) {
		o.transport = t
	}
}

func withAuth(auth authn.Authenticator) downloadOption {
	return func(o *downloadOptions) {
		o.auth = auth
	}
}

func withKeychain(k authn.Keychain) downloadOption {
	return func(o *downloadOptions) {
		o.keychain = k
	}
}

func withNumberOfChunks(n int) downloadOption {
	return func(o *downloadOptions) {
		o.numberOfChunks = n
	}
}

type chunk struct {
	n      int
	offset int64
	size   int64
	writeCounter
}

func makeChunk(n int, offset, size int64) *chunk {
	return &chunk{
		n:            n,
		offset:       offset,
		size:         size,
		writeCounter: writeCounter{},
	}
}

// newDownloader returns a new blobManager with the given options.
func newDownloader(name name.Reference, path string, layer v1.Layer, opts ...downloadOption) *blobManager {
	o := &downloadOptions{
		numberOfChunks: defaultNumberOfChunks,
		keychain:       authn.DefaultKeychain,
		transport:      remote.DefaultTransport.(*http.Transport).Clone(),
	}
	d := &blobManager{
		layer:           layer,
		name:            name,
		path:            path,
		downloadOptions: *o,
	}
	for _, opt := range opts {
		opt(&d.downloadOptions)
	}

	return d
}

func (d *blobManager) download(ctx context.Context) error {
	digest, err := d.layer.Digest()
	if err != nil {
		return fmt.Errorf("failed to get layer digest: %w", err)
	}
	d.digest = digest

	size, err := d.layer.Size()
	if err != nil {
		return fmt.Errorf("failed to get layer size: %w", err)
	}
	d.size = size

	if d.c == nil {
		h, err := makeHttpClient(ctx, d.name.Context(), &d.downloadOptions)
		if err != nil {
			return fmt.Errorf("failed to create HTTP client: %w", err)
		}
		d.c = h
	}

	ok, err := d.isRangeRequestEnabled(ctx)
	if err != nil {
		return fmt.Errorf("failed to check range request support: %w", err)
	}

	if !ok {
		return errRangeRequestNotSupported
	}

	if err := d.downloadChunks(ctx); err != nil {
		return fmt.Errorf("failed to download layer in chunks: %w", err)
	}

	if err := d.verifyDigest(); err != nil {
		return fmt.Errorf("failed to verify layer digest: %w", err)
	}

	return nil
}

func (d *blobManager) downloadChunks(ctx context.Context) error {
	u := makeUrl(d.name, d.digest)

	file, err := os.OpenFile(d.path+".tmp", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create layer file: %w", err)
	}
	defer file.Close()

	chunkSize := d.size / int64(d.numberOfChunks)
	if chunkSize < minChunkSize {
		chunkSize = minChunkSize
	} else if chunkSize > maxChunkSize {
		chunkSize = maxChunkSize
	}

	var (
		chunks []*chunk
		n      int
	)

	for offset := int64(0); offset < d.size; offset += chunkSize {
		if offset+chunkSize > d.size {
			chunkSize = d.size - offset
		}
		chunk := makeChunk(n, offset, chunkSize)
		chunks = append(chunks, chunk)
		n++
	}

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(d.numberOfChunks)
	for _, chunk := range chunks {
		chunk := chunk
		g.Go(func() error {
			b := defaultRetryBackoff
			for i := 0; i < retries; i++ {
				w := io.NewOffsetWriter(file, chunk.offset)
				err := chunk.download(ctx, d.c, w, u)
				switch {
				case errors.Is(err, context.Canceled), errors.Is(err, syscall.ENOSPC):
					return err
				case errors.Is(err, errCopyFailed):
					time.Sleep(b.Step())
					continue
				default:
					return nil
				}
			}
			return fmt.Errorf("failed to download chunk %d: %w", n, err)
		})
	}

	err = g.Wait()
	if err != nil {
		return fmt.Errorf("failed to download layer in chunks: %w", err)
	}

	if err := os.Rename(file.Name(), d.path); err != nil {
		return err
	}

	return nil

}

func (c *chunk) download(ctx context.Context, client *retryablehttp.Client, w io.Writer, u url.URL) error {
	req, err := retryablehttp.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}

	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", c.offset, c.offset+c.size-1))
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}

	if err := transport.CheckError(resp, http.StatusPartialContent); err != nil {
		return err
	}

	_, err = io.Copy(w, io.TeeReader(resp.Body, &c.writeCounter))
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.ErrUnexpectedEOF) {
		// TODO: if the download was interrupted, we can resume it
		return fmt.Errorf("failed to download chunk %d: %w", c.n, err)
	}

	return err
}

func (d *blobManager) isRangeRequestEnabled(ctx context.Context) (bool, error) {
	u := makeUrl(d.name, d.digest)
	req, err := retryablehttp.NewRequest(http.MethodHead, u.String(), nil)
	if err != nil {
		return false, err
	}

	resp, err := d.c.Do(req.WithContext(ctx))
	if err != nil {
		return false, err
	}

	if err := transport.CheckError(resp, http.StatusOK); err != nil {
		return false, err
	}

	if rangeUnit := resp.Header.Get("Accept-Ranges"); rangeUnit == "bytes" {
		return true, nil
	}

	return false, nil
}

func (d *blobManager) verifyDigest() error {
	f, err := os.Open(d.path)
	if err != nil {
		return fmt.Errorf("failed to open layer file: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return fmt.Errorf("failed to hash layer: %w", err)
	}

	newDigest := h.Sum(nil)
	if d.digest.String() != fmt.Sprintf("sha256:%x", newDigest) {
		return fmt.Errorf("layer digest does not match: %s != sha256:%x", d.digest.String(), newDigest)
	}
	return nil
}

func makeUrl(name name.Reference, digest v1.Hash) url.URL {
	return url.URL{
		Scheme: name.Context().Scheme(),
		Host:   name.Context().RegistryStr(),
		Path:   fmt.Sprintf("/v2/%s/blobs/%s", name.Context().RepositoryStr(), digest.String()),
	}
}

type resource interface {
	Scheme() string
	RegistryStr() string
	Scope(string) string

	authn.Resource
}

func makeHttpClient(ctx context.Context, target resource, o *downloadOptions) (*retryablehttp.Client, error) {
	auth := o.auth
	if o.keychain != nil {
		kauth, err := o.keychain.Resolve(target)
		if err != nil {
			return nil, err
		}
		auth = kauth
	}

	reg, ok := target.(name.Registry)
	if !ok {
		repo, ok := target.(name.Repository)
		if !ok {
			return nil, fmt.Errorf("unexpected resource: %T", target)
		}
		reg = repo.Registry
	}

	tr, err := transport.NewWithContext(ctx, reg, auth, o.transport, []string{target.Scope(transport.PullScope)})
	if err != nil {
		return nil, err
	}

	h := retryablehttp.NewClient()
	h.HTTPClient = &http.Client{Transport: tr}
	return h, nil
}
