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
	"time"

	"github.com/fluxcd/pkg/auth"
)

// AccessToken is a Docker Hub access token obtained by exchanging an OIDC
// identity.
type AccessToken struct {
	Token     string
	ExpiresAt time.Time
}

var _ auth.Credential = (*AccessToken)(nil)

// GetDuration implements auth.Credential.
func (a *AccessToken) GetDuration() time.Duration {
	return time.Until(a.ExpiresAt)
}
