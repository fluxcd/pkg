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
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func parseTokenResponse(body []byte) (*tokenResponse, error) {
	var token tokenResponse
	if err := json.Unmarshal(body, &token); err != nil {
		return nil, fmt.Errorf("failed to decode dockerhub token response: %w", err)
	}
	if token.AccessToken == "" {
		return nil, fmt.Errorf("dockerhub token response did not contain an access token")
	}
	return &token, nil
}

func apiError(statusCode int, body []byte) error {
	if len(body) > 0 {
		var payload any
		if err := json.Unmarshal(body, &payload); err == nil {
			return fmt.Errorf("dockerhub api: bad status code %d: %s", statusCode, strings.TrimSpace(string(body)))
		}
	}
	if statusCode == http.StatusUnauthorized {
		return fmt.Errorf("dockerhub api: operation not permitted")
	}
	return fmt.Errorf("dockerhub api: bad status code %d", statusCode)
}
