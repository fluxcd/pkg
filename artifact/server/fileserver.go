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

// Start starts a blocking HTTP file server using the provided configuration options.
// It supports graceful shutdown via the provided context (e.g., from ctrl.SetupSignalHandler).
// The Start function is meant to be run in a separate goroutine after the controller manager
// receives leadership (e.g., <-mgr.Elected()).
func Start(ctx context.Context, opts *config.Options) error {
	if opts == nil {
		return fmt.Errorf("options cannot be nil")
	}

	fs := http.FileServer(http.Dir(opts.StoragePath))
	mux := http.NewServeMux()
	mux.Handle("/", fs)

	server := &http.Server{
		Addr:    opts.StorageAddress,
		Handler: mux,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
