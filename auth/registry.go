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

import (
	"strings"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
)

// ArtifactRegistryCredentials is a particular type implementing the Token interface
// for credentials that can be used to authenticate against an artifact registry
// from a cloud provider. This type is compatible with all the cloud providers
// and should be returned when the artifact repository is configured in the options.
type ArtifactRegistryCredentials struct {
	authn.Authenticator
	ExpiresAt time.Time
}

func (a *ArtifactRegistryCredentials) GetDuration() time.Duration {
	return time.Until(a.ExpiresAt)
}

// GetRegistryFromArtifactRepository returns the registry from the artifact repository.
func GetRegistryFromArtifactRepository(artifactRepository string) (string, error) {
	registry := strings.TrimSuffix(artifactRepository, "/")
	if strings.ContainsRune(registry, '/') {
		ref, err := name.ParseReference(registry)
		if err != nil {
			return "", err
		}
		return ref.Context().RegistryStr(), nil
	}
	return registry, nil
}
