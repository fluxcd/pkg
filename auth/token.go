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

package auth

import "time"

// Token is an interface that represents an access token that can be used to
// authenticate requests for a cloud provider. The only common method is for
// getting the duration of the token, because different providers have
// different ways of representing the token. For example, Azure and GCP use
// a single string, while AWS uses three strings: access key ID, secret access
// key and token session. Consumers of this interface should know what type to
// cast it to.
type Token interface {
	// GetDuration returns the duration for which the token will still be valid
	// relative to approximately time.Now(). This is used to determine when the token should
	// be refreshed.
	GetDuration() time.Duration
}

// ArtifactRegistryCredentials is a particular type implementing the Token interface
// for credentials that can be used to authenticate with an artifact registry
// from a cloud provider. This type is compatible with all the cloud providers
// and should be returned when the artifact repository is configured in the options.
type ArtifactRegistryCredentials struct {
	Username  string
	Password  string
	ExpiresAt time.Time
}

func (a *ArtifactRegistryCredentials) GetDuration() time.Duration {
	return time.Until(a.ExpiresAt)
}
