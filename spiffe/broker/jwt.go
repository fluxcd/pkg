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
	"time"

	brokerapi "github.com/spiffe/go-spiffe/v2/exp/proto/spiffe/broker"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	// defaultBrokerFetchTimeout bounds a single Broker API fetch, including
	// retries, unless the caller's context has an earlier deadline.
	defaultBrokerFetchTimeout = 30 * time.Second
)

// FetchJWTSVID fetches a JWT-SVID for ref and the given audiences from the
// Broker. The first returned JWT-SVID is used. It retries transient failures
// with backoff until the context is done or the fetch timeout elapses.
// Permanent errors, such as PermissionDenied, are returned immediately.
func (c *client) FetchJWTSVID(ctx context.Context, ref KubernetesObjectReference,
	audiences []string) (*jwtsvid.SVID, error) {

	if len(audiences) == 0 {
		return nil, errors.New("at least one audience is required")
	}
	if ctx == nil {
		return nil, errors.New("context is required")
	}
	workload, err := ref.workloadReference()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, defaultBrokerFetchTimeout)
	defer cancel()

	for attempt := 1; ; attempt++ {
		svid, err := c.fetchJWTSVID(ctx, workload, audiences)
		if err == nil {
			return svid, nil
		}
		if !isRetryableError(err) {
			return nil, err
		}

		select {
		case <-time.After(fetchBackoff(attempt)):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *client) fetchJWTSVID(ctx context.Context, workload *brokerapi.WorkloadReference,
	audiences []string) (*jwtsvid.SVID, error) {

	// The Broker API requires this header on every request as an SSRF mitigation.
	ctx = metadata.AppendToOutgoingContext(ctx, "broker.spiffe.io", "true")
	resp, err := c.api.FetchJWTSVID(ctx, &brokerapi.FetchJWTSVIDRequest{
		Reference: workload,
		Audience:  audiences,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.GetSvids()) == 0 {
		return nil, permanentError{errors.New("SPIFFE Broker returned no JWT-SVIDs")}
	}

	svid, err := jwtsvid.ParseInsecure(resp.GetSvids()[0].GetSvid(), audiences)
	if err != nil {
		return nil, permanentError{fmt.Errorf("failed to parse JWT-SVID: %w", err)}
	}
	return svid, nil
}

// fetchBackoff returns the delay before the next fetch attempt.
func fetchBackoff(attempt int) time.Duration {
	d := time.Second * time.Duration(attempt)
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}

// permanentError marks an error that must not be retried.
type permanentError struct {
	err error
}

func (e permanentError) Error() string {
	return e.err.Error()
}

func (e permanentError) Unwrap() error {
	return e.err
}

// isRetryableError reports whether a Broker API error may succeed on retry.
func isRetryableError(err error) bool {
	if _, ok := errors.AsType[permanentError](err); ok {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	switch status.Code(err) {
	case codes.InvalidArgument, codes.Unauthenticated, codes.PermissionDenied,
		codes.NotFound, codes.FailedPrecondition, codes.Unimplemented:
		return false
	default:
		return true
	}
}
