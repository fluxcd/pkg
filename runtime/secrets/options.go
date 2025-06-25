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

package secrets

// Option is a functional option for configuring secret operations.
type Option func(*options)

// options holds configuration for secret operations.
type options struct {
	supportDeprecatedFields bool
}

// WithDeprecatedFieldSupport enables support for deprecated field names
// in TLS secrets (certFile, keyFile, caFile).
//
// When enabled, the function will check for deprecated field names as
// fallbacks if the standard field names (tls.crt, tls.key, ca.crt) are
// not found. Standard field names always take precedence.
func WithDeprecatedFieldSupport() Option {
	return func(o *options) {
		o.supportDeprecatedFields = true
	}
}

// makeOptions creates an options struct from a list of Option functions.
func makeOptions(opts []Option) *options {
	o := &options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}
