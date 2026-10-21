/*
Copyright 2026 The Flux authors

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

package spiffe_test

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"

	fluxspiffe "github.com/fluxcd/pkg/spiffe"
)

func TestBuildTLSConfig(t *testing.T) {
	g := NewWithT(t)

	id := spiffeid.RequireFromString("spiffe://example.org/controller")
	source, _ := newTestSource(t, id)

	tlsConfig, err := fluxspiffe.BuildTLSConfig(source, id.String())
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(tlsConfig.MinVersion).To(Equal(uint16(tls.VersionTLS13)))
	g.Expect(tlsConfig.ClientAuth).To(Equal(tls.RequireAnyClientCert))
	g.Expect(tlsConfig.GetCertificate).NotTo(BeNil())
	g.Expect(tlsConfig.GetClientCertificate).NotTo(BeNil())
	g.Expect(tlsConfig.VerifyPeerCertificate).NotTo(BeNil())

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	g.Expect(err).NotTo(HaveOccurred())
	defer listener.Close()

	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		serverErr <- tls.Server(conn, tlsConfig).Handshake()
	}()

	clientConn, err := tls.Dial("tcp", listener.Addr().String(), tlsConfig)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(clientConn.Close()).To(Succeed())
	g.Expect(<-serverErr).To(Succeed())
}

func TestBuildTLSConfigHTTP(t *testing.T) {
	g := NewWithT(t)

	id := spiffeid.RequireFromString("spiffe://example.org/controller")
	source, _ := newTestSource(t, id)

	tlsConfig, err := fluxspiffe.BuildTLSConfig(source, id.String())
	g.Expect(err).NotTo(HaveOccurred())

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	g.Expect(err).NotTo(HaveOccurred())

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("ok"))
		}),
		TLSConfig: tlsConfig,
	}
	go func() { _ = server.ServeTLS(listener, "", "") }()
	defer server.Close()

	client := &http.Client{
		Transport: &http.Transport{TLSClientConfig: tlsConfig},
		Timeout:   5 * time.Second,
	}
	defer client.CloseIdleConnections()

	resp, err := client.Get("https://" + listener.Addr().String())
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(string(body)).To(Equal("ok"))
}

func TestBuildTLSConfigValidation(t *testing.T) {
	g := NewWithT(t)

	id := spiffeid.RequireFromString("spiffe://example.org/controller")
	source, _ := newTestSource(t, id)

	_, err := fluxspiffe.BuildTLSConfig(nil, id.String())
	g.Expect(err).To(MatchError("SPIFFE TLS source is required"))

	_, err = fluxspiffe.BuildTLSConfig(source)
	g.Expect(err).To(MatchError("at least one SPIFFE peer is required"))

	_, err = fluxspiffe.BuildTLSConfig(source, "not-a-spiffe-id")
	g.Expect(err).To(MatchError(ContainSubstring("invalid SPIFFE peer at index 0")))

	_, err = fluxspiffe.BuildTLSConfig(source, id.String(), "spiffe://example.org/")
	g.Expect(err).To(MatchError(ContainSubstring("invalid SPIFFE peer at index 1")))

	g.Expect(source.Close()).To(Succeed())
	_, err = fluxspiffe.BuildTLSConfig(source, "spiffe://self")
	g.Expect(err).To(MatchError(ContainSubstring("failed to get SPIFFE X.509-SVID")))
}

func TestBuildTLSConfigAuthorizer(t *testing.T) {
	source, ca := newTestSource(t, spiffeid.RequireFromString("spiffe://example.org/source-controller"))

	tests := []struct {
		name         string
		peers        []string
		authorized   []string
		unauthorized []string
	}{
		{
			name:         "exact ID",
			peers:        []string{"spiffe://example.org/foo"},
			authorized:   []string{"spiffe://example.org/foo"},
			unauthorized: []string{"spiffe://example.org/bar", "spiffe://other.org/foo"},
		},
		{
			name:       "prefix",
			peers:      []string{"spiffe://example.org/ns/"},
			authorized: []string{"spiffe://example.org/ns", "spiffe://example.org/ns/foo", "spiffe://example.org/ns/foo/bar"},
			unauthorized: []string{
				"spiffe://example.org/nsfoo",
				"spiffe://other.org/ns/foo",
			},
		},
		{
			name:         "trust domain",
			peers:        []string{"spiffe://example.org"},
			authorized:   []string{"spiffe://example.org/foo", "spiffe://example.org/bar"},
			unauthorized: []string{"spiffe://other.org/foo"},
		},
		{
			name:         "self",
			peers:        []string{"spiffe://self"},
			authorized:   []string{"spiffe://example.org/kustomize-controller", "spiffe://example.org/helm-controller"},
			unauthorized: []string{"spiffe://other.org/kustomize-controller"},
		},
		{
			name:         "self exact ID",
			peers:        []string{"spiffe://self/ns/foo"},
			authorized:   []string{"spiffe://example.org/ns/foo"},
			unauthorized: []string{"spiffe://example.org/ns/bar", "spiffe://other.org/ns/foo"},
		},
		{
			name:         "self prefix",
			peers:        []string{"spiffe://self/ns/"},
			authorized:   []string{"spiffe://example.org/ns", "spiffe://example.org/ns/foo", "spiffe://example.org/ns/foo/bar"},
			unauthorized: []string{"spiffe://example.org/nsfoo", "spiffe://other.org/ns/foo"},
		},
		{
			name:       "multiple",
			peers:      []string{"spiffe://example.org/foo", "spiffe://example.org/ns/"},
			authorized: []string{"spiffe://example.org/foo", "spiffe://example.org/ns/bar"},
			unauthorized: []string{
				"spiffe://example.org/bar",
				"spiffe://example.org/nsfoo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			tlsConfig, err := fluxspiffe.BuildTLSConfig(source, tt.peers...)
			g.Expect(err).NotTo(HaveOccurred())

			for i, value := range tt.authorized {
				cert, _ := ca.newCertificate(t, spiffeid.RequireFromString(value), int64(100+i))
				g.Expect(tlsConfig.VerifyPeerCertificate([][]byte{cert.Raw}, nil)).
					To(Succeed(), "expected %s to be authorized", value)
			}
			for i, value := range tt.unauthorized {
				cert, _ := ca.newCertificate(t, spiffeid.RequireFromString(value), int64(200+i))
				g.Expect(tlsConfig.VerifyPeerCertificate([][]byte{cert.Raw}, nil)).
					NotTo(Succeed(), "expected %s not to be authorized", value)
			}
		})
	}
}

func TestBuildTLSConfigReconnects(t *testing.T) {
	g := NewWithT(t)

	id := spiffeid.RequireFromString("spiffe://example.org/source-controller")
	ca := newTestCA(t)
	handler := newFakeWorkloadAPI(ca.newX509SVID(t, id, 1))
	socketPath := newSocketPath(t)
	stop := startWorkloadAPI(t, socketPath, handler)

	t.Setenv(workloadapi.SocketEnv, "unix://"+socketPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(
		workloadapi.WithBackoffStrategy(fastBackoff{}),
	))
	g.Expect(err).NotTo(HaveOccurred())
	defer source.Close()

	tlsConfig, err := fluxspiffe.BuildTLSConfig(source, id.String())
	g.Expect(err).NotTo(HaveOccurred())

	cert, err := tlsConfig.GetClientCertificate(&tls.CertificateRequestInfo{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cert.Leaf.SerialNumber.Int64()).To(Equal(int64(1)))

	// Simulate a restart of the SPIFFE runtime by stopping the Workload API
	// server and starting it again with a rotated X.509-SVID.
	stop()
	handler.setX509SVID(ca.newX509SVID(t, id, 2))
	stop = startWorkloadAPI(t, socketPath, handler)
	defer stop()

	g.Eventually(func() int64 {
		cert, err := tlsConfig.GetClientCertificate(&tls.CertificateRequestInfo{})
		if err != nil {
			return -1
		}
		return cert.Leaf.SerialNumber.Int64()
	}, 5*time.Second, 50*time.Millisecond).Should(Equal(int64(2)))
}

func newTestSource(t *testing.T, id spiffeid.ID) (*workloadapi.X509Source, *testCA) {
	t.Helper()

	ca := newTestCA(t)
	handler := newFakeWorkloadAPI(ca.newX509SVID(t, id, 1))
	socketPath := newSocketPath(t)
	stop := startWorkloadAPI(t, socketPath, handler)
	t.Cleanup(stop)

	t.Setenv(workloadapi.SocketEnv, "unix://"+socketPath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(
		workloadapi.WithBackoffStrategy(fastBackoff{}),
	))
	if err != nil {
		t.Fatalf("failed to create SPIFFE X.509 source: %v", err)
	}
	t.Cleanup(func() { _ = source.Close() })
	return source, ca
}
