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

package server_test

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/artifact/config"
	"github.com/fluxcd/pkg/artifact/server"
)

func Test_Start(t *testing.T) {
	g := NewWithT(t)

	// Create temporary directory for storage
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "Hello, World!"
	err := os.WriteFile(testFile, []byte(testContent), 0644)
	g.Expect(err).NotTo(HaveOccurred())

	// Find available port
	listener, err := net.Listen("tcp", ":0")
	g.Expect(err).NotTo(HaveOccurred())
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	opts := &config.Options{
		StoragePath:    tmpDir,
		StorageAddress: fmt.Sprintf(":%d", port),
	}

	// Start server in goroutine with context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx, opts)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Test that server serves files
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/test.txt", port))
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	g.Expect(resp.StatusCode).To(Equal(http.StatusOK))

	body, err := io.ReadAll(resp.Body)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(body)).To(Equal(testContent))

	// Test directory listing (should work for file servers)
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/", port))
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	g.Expect(resp.StatusCode).To(Equal(http.StatusOK))

	// Verify no error from server yet
	select {
	case err := <-errCh:
		t.Fatalf("Server returned unexpected error: %v", err)
	default:
		// Good, no error yet
	}
}

func Test_Start_InvalidAddress(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	opts := &config.Options{
		StoragePath:    tmpDir,
		StorageAddress: "invalid:address:format",
	}

	ctx := context.Background()
	err := server.Start(ctx, opts)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("invalid:address:format"))
}

func Test_Start_AddressInUse(t *testing.T) {
	g := NewWithT(t)

	// Find available port and bind to it
	listener, err := net.Listen("tcp", ":0")
	g.Expect(err).NotTo(HaveOccurred())
	port := listener.Addr().(*net.TCPAddr).Port
	defer listener.Close()

	tmpDir := t.TempDir()
	opts := &config.Options{
		StoragePath:    tmpDir,
		StorageAddress: fmt.Sprintf(":%d", port),
	}

	// Try to start server on already bound port
	ctx := context.Background()
	err = server.Start(ctx, opts)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("bind"))
}

func Test_Start_GracefulShutdown(t *testing.T) {
	g := NewWithT(t)

	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	g.Expect(err).NotTo(HaveOccurred())

	// Find available port
	listener, err := net.Listen("tcp", ":0")
	g.Expect(err).NotTo(HaveOccurred())
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	opts := &config.Options{
		StoragePath:    tmpDir,
		StorageAddress: fmt.Sprintf(":%d", port),
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx, opts)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Verify server is running by making a request
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/test.txt", port))
	g.Expect(err).NotTo(HaveOccurred())
	resp.Body.Close()
	g.Expect(resp.StatusCode).To(Equal(http.StatusOK))

	// Cancel context to trigger graceful shutdown
	cancel()

	// Wait for server to shutdown
	select {
	case err := <-errCh:
		g.Expect(err).NotTo(HaveOccurred()) // Graceful shutdown should not return error
	case <-time.After(5 * time.Second):
		t.Fatal("Server did not shutdown within timeout")
	}

	// Verify server is no longer accepting connections
	_, err = http.Get(fmt.Sprintf("http://localhost:%d/test.txt", port))
	g.Expect(err).To(HaveOccurred())
}

func Test_Start_NilOptions(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()
	err := server.Start(ctx, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(Equal("options cannot be nil"))
}
