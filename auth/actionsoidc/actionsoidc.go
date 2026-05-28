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

// Package actionsoidc fetches OIDC ID tokens from the GitHub/Forgejo Actions
// token endpoint. Both providers expose the same contract: a job granted the
// 'id-token: write' permission gets the ACTIONS_ID_TOKEN_REQUEST_URL and
// ACTIONS_ID_TOKEN_REQUEST_TOKEN environment variables, and a GET request to
// that URL with the request token as a bearer credential returns a JSON object
// with the ID token in its "value" field.
package actionsoidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	// EnvRequestURL is the environment variable holding the URL of the
	// GitHub/Forgejo Actions OIDC token request endpoint.
	EnvRequestURL = "ACTIONS_ID_TOKEN_REQUEST_URL"

	// EnvRequestToken is the environment variable holding the bearer token used
	// to authenticate the request to the endpoint in EnvRequestURL.
	EnvRequestToken = "ACTIONS_ID_TOKEN_REQUEST_TOKEN"
)

// FetchToken requests an OIDC ID token for the given audience from the
// GitHub/Forgejo Actions token endpoint. The endpoint URL and the request
// bearer token are read from the EnvRequestURL and EnvRequestToken environment
// variables, which Actions injects into a job that has the 'id-token: write'
// permission.
func FetchToken(ctx context.Context, audience string) (string, error) {
	requestURL := os.Getenv(EnvRequestURL)
	requestToken := os.Getenv(EnvRequestToken)
	if requestURL == "" || requestToken == "" {
		return "", fmt.Errorf("%s and %s must be set in the environment; "+
			"ensure the Actions job has the 'id-token: write' permission", EnvRequestURL, EnvRequestToken)
	}

	u, err := url.Parse(requestURL)
	if err != nil {
		return "", fmt.Errorf("invalid %s: %w", EnvRequestURL, err)
	}
	q := u.Query()
	q.Set("audience", audience)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create OIDC token request: %w", err)
	}
	req.Header.Set("Authorization", "bearer "+requestToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("OIDC token request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OIDC token request failed with status %s: %s",
			resp.Status, strings.TrimSpace(string(body)))
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to decode OIDC token response: %w", err)
	}
	if result.Value == "" {
		return "", errors.New("the OIDC token response did not contain a token")
	}

	return result.Value, nil
}
