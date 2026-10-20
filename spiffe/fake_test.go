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
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc"
)

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}
	return &testCA{cert: cert, key: key}
}

func (ca *testCA) newCertificate(t *testing.T, id spiffeid.ID, serial int64) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate SVID key: %v", err)
	}
	uri, err := url.Parse(id.String())
	if err != nil {
		t.Fatalf("failed to parse SPIFFE ID: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		URIs:         []*url.URL{uri},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca.cert, &key.PublicKey, ca.key)
	if err != nil {
		t.Fatalf("failed to create SVID certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("failed to parse SVID certificate: %v", err)
	}
	return cert, key
}

func (ca *testCA) newX509SVID(t *testing.T, id spiffeid.ID, serial int64) *workload.X509SVID {
	t.Helper()

	cert, key := ca.newCertificate(t, id, serial)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("failed to marshal SVID key: %v", err)
	}
	return &workload.X509SVID{
		SpiffeId:    id.String(),
		X509Svid:    cert.Raw,
		X509SvidKey: keyDER,
		Bundle:      ca.cert.Raw,
	}
}

type fakeWorkloadAPI struct {
	workload.UnimplementedSpiffeWorkloadAPIServer

	mtx     sync.Mutex
	resp    *workload.X509SVIDResponse
	updated chan struct{}
}

func newFakeWorkloadAPI(svid *workload.X509SVID) *fakeWorkloadAPI {
	f := &fakeWorkloadAPI{updated: make(chan struct{})}
	f.setX509SVID(svid)
	return f
}

func (f *fakeWorkloadAPI) setX509SVID(svid *workload.X509SVID) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	f.resp = &workload.X509SVIDResponse{Svids: []*workload.X509SVID{svid}}
	close(f.updated)
	f.updated = make(chan struct{})
}

func (f *fakeWorkloadAPI) FetchX509SVID(_ *workload.X509SVIDRequest,
	stream grpc.ServerStreamingServer[workload.X509SVIDResponse]) error {
	for {
		f.mtx.Lock()
		resp := f.resp
		updated := f.updated
		f.mtx.Unlock()

		if err := stream.Send(resp); err != nil {
			return err
		}

		select {
		case <-updated:
		case <-stream.Context().Done():
			return nil
		}
	}
}

func newSocketPath(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "flux-spiffe")
	if err != nil {
		t.Fatalf("failed to create temporary directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir + "/api.sock"
}

func startWorkloadAPI(t *testing.T, socketPath string, handler *fakeWorkloadAPI) (stop func()) {
	t.Helper()

	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed to remove stale socket: %v", err)
	}
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("failed to listen on socket: %v", err)
	}
	server := grpc.NewServer()
	workload.RegisterSpiffeWorkloadAPIServer(server, handler)
	go func() { _ = server.Serve(listener) }()

	var once sync.Once
	stop = func() {
		once.Do(func() {
			server.Stop()
			_ = listener.Close()
			_ = os.Remove(socketPath)
		})
	}
	t.Cleanup(stop)
	return stop
}

type fastBackoff struct{}

func (fastBackoff) NewBackoff() workloadapi.Backoff {
	return fastBackoffInstance{}
}

type fastBackoffInstance struct{}

func (fastBackoffInstance) Next() time.Duration { return 10 * time.Millisecond }
func (fastBackoffInstance) Reset()              {}
