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
	"fmt"
	"net/url"

	"github.com/spf13/pflag"
	"golang.org/x/net/http/httpproxy"
)

const (
	flagInsecureAllowHTTP = "insecure-allow-http"
)

// ConnectionOptions defines the configurable options for outbound connections
// opened by reconcilers. Consumers are expected to check for compatibility via
// `CheckEnvironmentCompatibility()` before using its values.
type ConnectionOptions struct {
	// AllowHTTP, if set to true allows the controller to make plain HTTP
	// connections to external services.
	AllowHTTP bool
}

// BindFlags will parse the given pflag.FlagSet for the controller and
// set the ConnectionOptions accordingly.
func (o *ConnectionOptions) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.AllowHTTP, flagInsecureAllowHTTP, true,
		"Allow the controller to make HTTP requests to external services like insecure Git servers, container registries, etc.")
}

// CheckEnvironmentCompatibility checks if the enviornment is compatible with
// the configured connection options.
func (o *ConnectionOptions) CheckEnvironmentCompatibility() error {
	if !o.AllowHTTP {
		config := httpproxy.FromEnvironment()
		if config.HTTPProxy != "" {
			return fmt.Errorf("usage of HTTP requests is blocked but found a HTTP proxy set in enviornment")
		}

		if config.HTTPSProxy != "" {
			proxy, err := url.Parse(config.HTTPSProxy)
			if err != nil {
				return fmt.Errorf("unable to parse address specified in the HTTPS proxy enviornment setting: %w", err)
			}

			if proxy.Scheme != "https" {
				return fmt.Errorf("usage of HTTP requests is blocked but found a non-https address in the HTTPS proxy enviornment setting")
			}
		}
	}
	return nil
}
