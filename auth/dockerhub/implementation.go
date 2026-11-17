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

package dockerhub

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxRetries = 5

// Implementation performs the Docker Hub HTTP exchange. It exists so tests can
// substitute the transport layer.
type Implementation interface {
	ExchangeToken(ctx context.Context, client *http.Client, endpoint string, form url.Values) (*http.Response, error)
}

type implementation struct{}

func (implementation) ExchangeToken(ctx context.Context, client *http.Client, endpoint string,
	form url.Values) (*http.Response, error) {

	body := form.Encode()
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create dockerhub token request: %w", err)
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("dockerhub token request failed: %w", err)
		}
		if resp.StatusCode != http.StatusTooManyRequests || attempt >= maxRetries {
			return resp, nil
		}
		delay, ok := retryAfter(resp.Header.Get("Retry-After"))
		if !ok {
			return resp, nil
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

// retryAfter parses the numeric seconds form of Retry-After.
func retryAfter(value string) (time.Duration, bool) {
	seconds, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, false
	}
	if seconds < 0 {
		seconds = 0
	}
	return time.Duration(seconds) * time.Second, true
}
