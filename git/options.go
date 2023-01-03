/*
Copyright 2021 The Flux authors

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

package git

import (
	"fmt"
	"net/url"
)

const (
	DefaultRemote            = "origin"
	DefaultBranch            = "master"
	DefaultPublicKeyAuthUser = "git"
)

type TransportType string

const (
	SSH   TransportType = "ssh"
	HTTPS TransportType = "https"
	HTTP  TransportType = "http"
)

// AuthOptions are the authentication options for the Transport of
// communication with a remote origin.
type AuthOptions struct {
	Transport   TransportType
	Host        string
	Username    string
	Password    string
	BearerToken string
	Identity    []byte
	KnownHosts  []byte
	CAFile      []byte
}

// KexAlgos hosts the key exchange algorithms to be used for SSH connections.
// If empty, Go's default is used instead.
var KexAlgos []string

// HostKeyAlgos holds the HostKey algorithms that the SSH client will advertise
// to the server. If empty, Go's default is used instead.
var HostKeyAlgos []string

// Validate the AuthOptions against the defined Transport.
func (o AuthOptions) Validate() error {
	switch o.Transport {
	case HTTPS, HTTP:
		if o.Username == "" && o.Password != "" {
			return fmt.Errorf("invalid '%s' auth option: 'password' requires 'username' to be set", o.Transport)
		}
	case SSH:
		if o.Host == "" {
			return fmt.Errorf("invalid '%s' auth option: 'host' is required", o.Transport)
		}
		if len(o.Identity) == 0 {
			return fmt.Errorf("invalid '%s' auth option: 'identity' is required", o.Transport)
		}
		if len(o.KnownHosts) == 0 {
			return fmt.Errorf("invalid '%s' auth option: 'known_hosts' is required", o.Transport)
		}
	case "":
		return fmt.Errorf("no transport type set")
	default:
		return fmt.Errorf("unknown transport '%s'", o.Transport)
	}
	return nil
}

// NewAuthOptions constructs an AuthOptions object from the given map and URL.
// If the map is empty, it returns a minimal AuthOptions object after
// validating the result.
func NewAuthOptions(u url.URL, data map[string][]byte) (*AuthOptions, error) {
	opts := newAuthOptions(u)
	if len(data) > 0 {
		opts.Username = string(data["username"])
		opts.Password = string(data["password"])
		opts.BearerToken = string(data["bearerToken"])
		opts.CAFile = data["caFile"]
		opts.Identity = data["identity"]
		opts.KnownHosts = data["known_hosts"]
	}

	if opts.Username == "" {
		opts.Username = u.User.Username()
	}

	// We fallback to using "git" as the username when cloning Git
	// repositories through SSH since that's the conventional username used
	// by Git providers.
	if opts.Username == "" && opts.Transport == SSH {
		opts.Username = DefaultPublicKeyAuthUser
	}
	if opts.Password == "" {
		opts.Password, _ = u.User.Password()
	}

	if err := opts.Validate(); err != nil {
		return nil, err
	}

	return opts, nil
}

// newAuthOptions returns a minimal AuthOptions object constructed from
// the given URL.
func newAuthOptions(u url.URL) *AuthOptions {
	opts := &AuthOptions{
		Transport: TransportType(u.Scheme),
		Host:      u.Host,
	}

	return opts
}
