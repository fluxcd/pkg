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

package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/fluxcd/pkg/artifact/config"
)

const (
	// readHeaderTimeout bounds the TLS handshake and the reading of the
	// request headers.
	readHeaderTimeout = 10 * time.Second

	// idleTimeout bounds how long a keep-alive connection is kept open
	// between requests. It is longer than the 90s idle timeout of the Flux
	// HTTP clients, so that clients usually close idle connections first.
	idleTimeout = 120 * time.Second
)

// newServer returns the HTTP server for the given options. The server sets
// no read or write timeout for the whole request, so that large artifacts
// can be downloaded by slow clients.
func newServer(opts *config.Options) *http.Server {
	fs := http.FileServer(http.Dir(opts.StoragePath))
	mux := http.NewServeMux()
	mux.Handle("/", fs)

	return &http.Server{
		Addr:              opts.StorageAddress,
		Handler:           mux,
		TLSConfig:         opts.TLSConfig,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// Start starts a blocking HTTP or HTTPS file server using the provided
// configuration options. When opts.TLSConfig is nil, the server serves plain
// HTTP. It supports graceful shutdown via the provided context (e.g., from
// ctrl.SetupSignalHandler). The Start function is meant to be run in a separate
// goroutine after the controller manager receives leadership (e.g.,
// <-mgr.Elected()).
func Start(ctx context.Context, opts *config.Options) error {
	if opts == nil {
		return fmt.Errorf("options cannot be nil")
	}

	server := newServer(opts)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		var err error
		if opts.TLSConfig != nil {
			err = server.ListenAndServeTLS("", "")
		} else {
			err = server.ListenAndServe()
		}
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		// Graceful shutdown with timeout
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
