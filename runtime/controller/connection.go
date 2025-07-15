/*
Copyright 2023 The Flux authors

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

package controller

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/spf13/pflag"
	"golang.org/x/net/http/httpproxy"
)

const (
	flagInsecureAllowHTTP = "insecure-allow-http"
)

// ErrInsecureHTTPBlocked signals that the use of insecure plain HTTP
// connections was requested but such behavior is blocked.
var ErrInsecureHTTPBlocked = errors.New("use of insecure plain HTTP connections is blocked")

// ConnectionOptions defines the configurable options for outbound connections
// opened by reconcilers. Consumers are expected to check for compatibility via
// `CheckEnvironmentCompatibility()` before using its values.
type ConnectionOptions struct {
	// AllowHTTP, if set to true allows the controller to make plain HTTP
	// connections to external services.
	AllowHTTP bool

	// ProxyFromEnvironment is a function that returns the proxy configuration
	// from the environment. If not specified, defaults to httpproxy.FromEnvironment.
	ProxyFromEnvironment func() *httpproxy.Config
}

// BindFlags will parse the given pflag.FlagSet for the controller and
// set the ConnectionOptions accordingly.
func (o *ConnectionOptions) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.AllowHTTP, flagInsecureAllowHTTP, true,
		"Allow the controller to make HTTP requests to external services like insecure Git servers, container registries, etc.")
}

// CheckEnvironmentCompatibility checks if the environment is compatible with
// the configured connection options.
func (o *ConnectionOptions) CheckEnvironmentCompatibility() error {
	if !o.AllowHTTP {
		proxyFromEnvironment := o.ProxyFromEnvironment
		if proxyFromEnvironment == nil {
			proxyFromEnvironment = httpproxy.FromEnvironment
		}
		config := proxyFromEnvironment()
		if config.HTTPProxy != "" {
			return fmt.Errorf("%w: found HTTP proxy set in environment", ErrInsecureHTTPBlocked)
		}

		if config.HTTPSProxy != "" {
			proxy, err := url.Parse(config.HTTPSProxy)
			if err != nil {
				return fmt.Errorf("unable to parse address specified in the HTTPS proxy environment setting: %w", err)
			}

			if proxy.Scheme != "https" {
				return fmt.Errorf("%w: found a non-https address in HTTPS proxy environment setting", ErrInsecureHTTPBlocked)
			}
		}
	}
	return nil
}
