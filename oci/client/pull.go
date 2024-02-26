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

package client

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/fluxcd/pkg/tar"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/hashicorp/go-retryablehttp"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"golang.org/x/sync/errgroup"
)

const (
	// thresholdForConcurrentPull is the maximum size of a layer to be extracted in one go.
	// If the layer is larger than this, it will be downloaded in chunks.
	thresholdForConcurrentPull = 100 * 1024 * 1024 // 100MB
	// maxConcurrentPulls is the maximum number of concurrent downloads.
	maxConcurrentPulls = 10
)

var (
	// gzipMagicHeader are bytes found at the start of gzip files
	// https://github.com/google/go-containerregistry/blob/a54d64203cffcbf94146e04069aae4a97f228ee2/internal/gzip/zip.go#L28
	gzipMagicHeader = []byte{'\x1f', '\x8b'}
)

// PullOptions contains options for pulling a layer.
type PullOptions struct {
	layerIndex  int
	layerType   LayerType
	transport   http.RoundTripper
	auth        authn.Authenticator
	keychain    authn.Keychain
	concurrency int
}

// PullOption is a function for configuring PullOptions.
type PullOption func(o *PullOptions)

// WithPullLayerType sets the layer type of the layer that is being pulled.
func WithPullLayerType(l LayerType) PullOption {
	return func(o *PullOptions) {
		o.layerType = l
	}
}

// WithPullLayerIndex set the index of the layer to be pulled.
func WithPullLayerIndex(i int) PullOption {
	return func(o *PullOptions) {
		o.layerIndex = i
	}
}

func WithTransport(t http.RoundTripper) PullOption {
	return func(o *PullOptions) {
		o.transport = t
	}
}

func WithConcurrency(c int) PullOption {
	return func(o *PullOptions) {
		o.concurrency = c
	}
}

// Pull downloads an artifact from an OCI repository and extracts the content.
// It untar or copies the content to the given outPath depending on the layerType.
// If no layer type is given, it tries to determine the right type by checking compressed content of the layer.
func (c *Client) Pull(ctx context.Context, urlString, outPath string, opts ...PullOption) (*Metadata, error) {
	o := &PullOptions{
		layerIndex: 0,
	}
	o.keychain = authn.DefaultKeychain
	for _, opt := range opts {
		opt(o)
	}

	if o.concurrency == 0 || o.concurrency > maxConcurrentPulls {
		o.concurrency = maxConcurrentPulls
	}

	if o.transport == nil {
		transport := remote.DefaultTransport.(*http.Transport).Clone()
		o.transport = transport
	}

	ref, err := name.ParseReference(urlString)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	if c.httpClient == nil {
		h, err := makeHttpClient(ctx, ref.Context(), *o)
		if err != nil {
			return nil, err
		}
		c.httpClient = h
	}

	img, err := crane.Pull(urlString, c.optionsWithContext(ctx)...)
	if err != nil {
		return nil, err
	}

	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("parsing digest failed: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("parsing manifest failed: %w", err)
	}

	meta := MetadataFromAnnotations(manifest.Annotations)
	meta.URL = urlString
	meta.Digest = ref.Context().Digest(digest.String()).String()

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to list layers: %w", err)
	}

	if len(layers) < 1 {
		return nil, fmt.Errorf("no layers found in artifact")
	}

	if len(layers) < o.layerIndex+1 {
		return nil, fmt.Errorf("index '%d' out of bound for '%d' layers in artifact", o.layerIndex, len(layers))
	}

	size, err := layers[o.layerIndex].Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get layer size: %w", err)
	}

	if size > thresholdForConcurrentPull {
		digest, err := layers[o.layerIndex].Digest()
		if err != nil {
			return nil, fmt.Errorf("parsing digest failed: %w", err)
		}
		u := url.URL{
			Scheme: ref.Context().Scheme(),
			Host:   ref.Context().RegistryStr(),
			Path:   fmt.Sprintf("/v2/%s/blobs/%s", ref.Context().RepositoryStr(), digest.String()),
		}
		ok, err := c.IsRangeRequestEnabled(ctx, u)
		if err != nil {
			return nil, fmt.Errorf("failed to check range request support: %w", err)
		}
		if ok {
			err = c.concurrentExtractLayer(ctx, u, layers[o.layerIndex], outPath, digest, size, o.concurrency)
			if err != nil {
				return nil, err
			}
			return meta, nil
		}
	}

	err = extractLayer(layers[o.layerIndex], outPath, o.layerType)
	if err != nil {
		return nil, err
	}
	return meta, nil
}

// TO DO: handle authentication handle using keychain for authentication
func (c *Client) IsRangeRequestEnabled(ctx context.Context, u url.URL) (bool, error) {
	req, err := retryablehttp.NewRequest(http.MethodHead, u.String(), nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req.WithContext(ctx))
	if err != nil {
		return false, err
	}

	if err := transport.CheckError(resp, http.StatusOK); err != nil {
		return false, err
	}

	if rangeUnit := resp.Header.Get("Accept-Ranges"); rangeUnit == "bytes" {
		return true, nil
	}
	for k, v := range resp.Header {
		fmt.Printf("Header: %s, Value: %s\n", k, v)
	}
	return false, nil
}

func (c *Client) concurrentExtractLayer(ctx context.Context, u url.URL, layer v1.Layer, path string, digest v1.Hash, size int64, concurrency int) error {
	chunkSize := size / int64(concurrency)
	chunks := make([][]byte, concurrency+1)
	diff := size % int64(concurrency)

	g, ctx := errgroup.WithContext(ctx)
	for i := 0; i < concurrency; i++ {
		i := i
		g.Go(func() (err error) {
			start, end := int64(i)*chunkSize, int64(i+1)*chunkSize
			if i == concurrency-1 {
				end += diff
			}
			req, err := retryablehttp.NewRequest(http.MethodGet, u.String(), nil)
			if err != nil {
				return fmt.Errorf("failed to create a new request: %w", err)
			}
			req.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", start, end-1))
			resp, err := c.httpClient.Do(req.WithContext(ctx))
			if err != nil {
				return fmt.Errorf("failed to download archive: %w", err)
			}
			defer resp.Body.Close()

			if err := transport.CheckError(resp, http.StatusPartialContent); err != nil {
				return fmt.Errorf("failed to download archive from %s (status: %s)", u.String(), resp.Status)
			}

			c, err := io.ReadAll(io.LimitReader(resp.Body, end-start))
			if err != nil {
				return fmt.Errorf("failed to read response body: %w", err)
			}
			chunks[i] = c
			return nil
		})
	}
	err := g.Wait()
	if err != nil {
		return err
	}

	content := bufio.NewReader(bytes.NewReader(bytes.Join(chunks, nil)))
	d, s, err := v1.SHA256(content)
	if err != nil {
		return err
	}
	if d != digest {
		return fmt.Errorf("digest mismatch: expected %s, got %s", digest, d)
	}
	if s != size {
		return fmt.Errorf("size mismatch: expected %d, got %d", size, size)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}

	_, err = io.Copy(f, content)
	if err != nil {
		return fmt.Errorf("error copying layer content: %s", err)
	}
	return nil
}

// extractLayer extracts the Layer to the path
func extractLayer(layer v1.Layer, path string, layerType LayerType) error {
	var blob io.Reader
	blob, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("extracting layer failed: %w", err)
	}

	actualLayerType := layerType
	if actualLayerType == "" {
		bufReader := bufio.NewReader(blob)
		if ok, _ := isGzipBlob(bufReader); ok {
			actualLayerType = LayerTypeTarball
		} else {
			actualLayerType = LayerTypeStatic
		}
		// the bufio.Reader has read the bytes from the io.Reader
		// and should be used instead
		blob = bufReader
	}

	return extractLayerType(path, blob, actualLayerType)
}

// extractLayerType extracts the contents of a io.Reader to the given path.
// If the LayerType is LayerTypeTarball, it will untar to a directory,
// If the LayerType is LayerTypeStatic, it will copy to a file.
func extractLayerType(path string, blob io.Reader, layerType LayerType) error {
	switch layerType {
	case LayerTypeTarball:
		return tar.Untar(blob, path, tar.WithMaxUntarSize(-1), tar.WithSkipSymlinks())
	case LayerTypeStatic:
		f, err := os.Create(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(f, blob)
		if err != nil {
			return fmt.Errorf("error copying layer content: %s", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported layer type: '%s'", layerType)
	}
}

// isGzipBlob reads the first two bytes from a bufio.Reader and
// checks that they are equal to the expected gzip file headers.
func isGzipBlob(buf *bufio.Reader) (bool, error) {
	b, err := buf.Peek(len(gzipMagicHeader))
	if err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	return bytes.Equal(b, gzipMagicHeader), nil
}

type resource interface {
	Scheme() string
	RegistryStr() string
	Scope(string) string

	authn.Resource
}

func makeHttpClient(ctx context.Context, target resource, o PullOptions) (*retryablehttp.Client, error) {
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
