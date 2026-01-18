/*
Copyright 2022 The Flux authors

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

package oci

import (
	"errors"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
)

// LoginWithCredentials configures the client with static credentials, accepts a single token
// or a user:password format.
func (c *Client) LoginWithCredentials(credentials string) error {
	auth, err := GetAuthFromCredentials(credentials)
	if err != nil {
		return err
	}

	c.options = append(c.options, crane.WithAuth(auth))
	return nil
}

// GetAuthFromCredentials returns an authn.Authenticator for the static credentials, accepts a single token
// or a user:password format.
func GetAuthFromCredentials(credentials string) (authn.Authenticator, error) {
	if credentials == "" {
		return nil, errors.New("credentials cannot be empty")
	}

	switch parts := strings.SplitN(credentials, ":", 2); {
	case len(parts) == 1:
		return &authn.Bearer{Token: parts[0]}, nil
	default:
		return &authn.Basic{Username: parts[0], Password: parts[1]}, nil
	}
}
