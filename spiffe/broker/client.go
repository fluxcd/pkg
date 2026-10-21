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

package broker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	brokerapi "github.com/spiffe/go-spiffe/v2/exp/proto/spiffe/broker"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/fluxcd/pkg/spiffe"
)

const (
	// roundRobinServiceConfig makes gRPC load-balance across the endpoints
	// returned by the DNS resolver instead of pinning to a single one.
	roundRobinServiceConfig = `{"loadBalancingConfig":[{"round_robin":{}}]}`
)

// Interface provides the operations used to acquire JWT-SVIDs from the SPIFFE
// Broker API.
type Interface interface {
	// FetchJWTSVID fetches a JWT-SVID for ref and the given audiences from the
	// Broker. The first returned JWT-SVID is used.
	FetchJWTSVID(ctx context.Context, ref KubernetesObjectReference, audiences []string) (*jwtsvid.SVID, error)
}

// Client is a SPIFFE Broker API client that also owns its connection.
type Client interface {
	Interface

	// Close closes the connection to the Broker endpoint.
	Close() error
}

// client owns a single gRPC connection to the Broker endpoint, which is
// automatically re-established by gRPC when it breaks.
type client struct {
	conn *grpc.ClientConn
	api  brokerapi.APIClient
}

var _ Client = (*client)(nil)

// New returns a client for the SPIFFE Broker API at endpoint. The endpoint must
// be a TCP gRPC target, e.g.
// dns:///spire-server.spire.svc.cluster.local:443. A target without a scheme is
// resolved with the dns resolver so that headless Kubernetes Services can be
// load-balanced across endpoints.
//
// The connection authenticates with the X.509-SVID from source and authorizes
// the Broker endpoint with any of brokerIDs. Each ID may be an exact SPIFFE ID
// or an ID prefix, as parsed by cryptov1.ParseSPIFFEID.
func New(endpoint string, brokerIDs []string, source spiffe.TLSSource) (Client, error) {
	if endpoint == "" {
		return nil, errors.New("SPIFFE Broker endpoint is required")
	}
	if len(brokerIDs) == 0 {
		return nil, errors.New("at least one SPIFFE Broker ID is required")
	}
	for i, id := range brokerIDs {
		if id == "" {
			return nil, fmt.Errorf("SPIFFE Broker ID at index %d is empty", i)
		}
	}
	if source == nil {
		return nil, errors.New("SPIFFE TLS source is required")
	}

	tlsConfig, err := spiffe.BuildTLSConfig(source, brokerIDs...)
	if err != nil {
		return nil, err
	}

	conn, err := grpc.NewClient(brokerTarget(endpoint),
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
		grpc.WithDefaultServiceConfig(roundRobinServiceConfig),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIFFE Broker client: %w", err)
	}

	return &client{
		conn: conn,
		api:  brokerapi.NewAPIClient(conn),
	}, nil
}

// Close closes the connection to the Broker endpoint.
func (c *client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// brokerTarget defaults a bare host:port target to the DNS resolver so that
// gRPC can discover and load-balance across endpoints. Targets that already
// carry a scheme are returned unchanged.
func brokerTarget(endpoint string) string {
	if strings.Contains(endpoint, "://") {
		return endpoint
	}
	return "dns:///" + endpoint
}
