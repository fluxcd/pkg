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

package utils

import (
	"github.com/fluxcd/pkg/auth"
)

// Option is a functional option for the utils package.
type Option func(*options)

type options struct {
	defaultServiceAccount string
	authOpts              []auth.Option
}

// WithDefaultServiceAccount sets the default ServiceAccount name to use for
// object-level workload identity when the kubeconfig ConfigMap does not specify
// one. Controllers pass the value of the flag relevant to the operation being
// performed.
func WithDefaultServiceAccount(name string) Option {
	return func(o *options) {
		o.defaultServiceAccount = name
	}
}

// WithAuthOptions appends the auth options to use when retrieving credentials.
func WithAuthOptions(opts ...auth.Option) Option {
	return func(o *options) {
		o.authOpts = append(o.authOpts, opts...)
	}
}
