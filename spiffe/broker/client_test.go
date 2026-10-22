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

package broker_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"net"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/onsi/gomega"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	brokerapi "github.com/spiffe/go-spiffe/v2/exp/proto/spiffe/broker"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	fluxspiffe "github.com/fluxcd/pkg/spiffe"
	fluxbroker "github.com/fluxcd/pkg/spiffe/broker"
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

type staticTLSSource struct {
	svid    *x509svid.SVID
	bundles *x509bundle.Set
}

func newStaticTLSSource(t *testing.T, ca *testCA, id spiffeid.ID, serial int64) *staticTLSSource {
	t.Helper()

	cert, key := ca.newCertificate(t, id, serial)
	bundle := x509bundle.FromX509Authorities(id.TrustDomain(), []*x509.Certificate{ca.cert})
	return &staticTLSSource{
		svid: &x509svid.SVID{
			ID:           id,
			Certificates: []*x509.Certificate{cert},
			PrivateKey:   key,
		},
		bundles: x509bundle.NewSet(bundle),
	}
}

func (s *staticTLSSource) GetX509SVID() (*x509svid.SVID, error) {
	return s.svid, nil
}

func (s *staticTLSSource) GetX509BundleForTrustDomain(trustDomain spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	return s.bundles.GetX509BundleForTrustDomain(trustDomain)
}

type fakeBrokerAPI struct {
	brokerapi.UnimplementedAPIServer

	mtx sync.Mutex
	jwt *brokerapi.FetchJWTSVIDResponse
	err error

	jwtFetches atomic.Int64
}

func (f *fakeBrokerAPI) setJWT(token string) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	f.jwt = &brokerapi.FetchJWTSVIDResponse{
		Svids: []*brokerapi.JWTSVID{{Svid: token}},
	}
	f.err = nil
}

func (f *fakeBrokerAPI) setError(err error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()
	f.err = err
}

func (f *fakeBrokerAPI) FetchJWTSVID(context.Context,
	*brokerapi.FetchJWTSVIDRequest) (*brokerapi.FetchJWTSVIDResponse, error) {

	f.jwtFetches.Add(1)
	f.mtx.Lock()
	defer f.mtx.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.jwt, nil
}

func startBrokerAPI(t *testing.T, handler *fakeBrokerAPI, serverTLS *tls.Config) net.Addr {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(serverTLS)))
	brokerapi.RegisterAPIServer(server, handler)
	go func() { _ = server.Serve(listener) }()

	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	return listener.Addr()
}

func newClientFixture(t *testing.T) (client fluxbroker.Client, handler *fakeBrokerAPI,
	ref fluxbroker.KubernetesObjectReference) {

	t.Helper()

	sourceID := spiffeid.RequireFromString("spiffe://example.org/source-controller")
	brokerID := spiffeid.RequireFromString("spiffe://example.org/spire-agent")
	ca := newTestCA(t)

	clientSource := newStaticTLSSource(t, ca, sourceID, 1)
	serverSource := newStaticTLSSource(t, ca, brokerID, 2)
	serverTLS, err := fluxspiffe.BuildTLSConfig(serverSource, sourceID.String())
	if err != nil {
		t.Fatalf("failed to build server TLS config: %v", err)
	}

	handler = &fakeBrokerAPI{}
	listenerAddr := startBrokerAPI(t, handler, serverTLS)

	// The server identifies itself with brokerID. The second value is another
	// allowed Broker ID, so this also exercises multiple --spiffe-broker-id
	// values.
	brokerIDs := []string{"spiffe://example.org/other-spire-agent", brokerID.String()}
	client, err = fluxbroker.New(listenerAddr.String(), brokerIDs, clientSource)
	if err != nil {
		t.Fatalf("failed to create Broker client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	ref = fluxbroker.KubernetesObjectReference{
		Group:     "source.toolkit.fluxcd.io",
		Kind:      "OCIRepository",
		Namespace: "flux-system",
		Name:      "my-repo",
		UID:       "b7b1b3b4-0000-0000-0000-000000000000",
	}
	return client, handler, ref
}

func TestNewValidatesBrokerIDs(t *testing.T) {
	g := gomega.NewWithT(t)

	_, err := fluxbroker.New("127.0.0.1:1", nil, nil)
	g.Expect(err).To(gomega.MatchError("at least one SPIFFE Broker ID is required"))

	_, err = fluxbroker.New("127.0.0.1:1", []string{"spiffe://example.org/agent", ""}, nil)
	g.Expect(err).To(gomega.MatchError("SPIFFE Broker ID at index 1 is empty"))
}

func TestFetchJWTSVID(t *testing.T) {
	g := gomega.NewWithT(t)
	client, handler, ref := newClientFixture(t)

	handler.setJWT(newTestJWTSVID(t, spiffeid.RequireFromString("spiffe://example.org/ns/foo"),
		[]string{"my-audience"}, time.Hour))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	svid, err := client.FetchJWTSVID(ctx, ref, []string{"my-audience"})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(svid.Expiry).To(gomega.BeTemporally(">", time.Now()))
	g.Expect(handler.jwtFetches.Load()).To(gomega.Equal(int64(1)))
}

func TestFetchJWTSVIDRetries(t *testing.T) {
	g := gomega.NewWithT(t)
	client, handler, ref := newClientFixture(t)

	handler.setError(status.Error(codes.Unavailable, "down"))
	go func() {
		time.Sleep(50 * time.Millisecond)
		handler.setJWT(newTestJWTSVID(t, spiffeid.RequireFromString("spiffe://example.org/ns/foo"),
			[]string{"my-audience"}, time.Hour))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.FetchJWTSVID(ctx, ref, []string{"my-audience"})
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(handler.jwtFetches.Load()).To(gomega.BeNumerically(">", 1))
}

func TestFetchJWTSVIDPermanentError(t *testing.T) {
	g := gomega.NewWithT(t)
	client, handler, ref := newClientFixture(t)

	handler.setError(status.Error(codes.PermissionDenied, "denied"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := client.FetchJWTSVID(ctx, ref, []string{"my-audience"})
	g.Expect(status.Code(err)).To(gomega.Equal(codes.PermissionDenied))
	g.Expect(handler.jwtFetches.Load()).To(gomega.Equal(int64(1)))
}

func newTestJWTSVID(t *testing.T, id spiffeid.ID, audiences []string, ttl time.Duration) string {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate JWT key: %v", err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, nil)
	if err != nil {
		t.Fatalf("failed to create JWT signer: %v", err)
	}
	claims, err := json.Marshal(map[string]any{
		"sub": id.String(),
		"aud": audiences,
		"exp": time.Now().Add(ttl).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	})
	if err != nil {
		t.Fatalf("failed to marshal JWT claims: %v", err)
	}
	jws, err := signer.Sign(claims)
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	token, err := jws.CompactSerialize()
	if err != nil {
		t.Fatalf("failed to serialize JWT: %v", err)
	}
	return token
}
