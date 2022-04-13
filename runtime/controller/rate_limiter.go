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

package controller

import (
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
)

const (
	defaultMinRetryDelay = 750 * time.Millisecond
	defaultMaxRetryDelay = 15 * time.Minute
	flagMinRetryDelay    = "min-retry-delay"
	flagMaxRetryDelay    = "max-retry-delay"
)

// RateLimiterOptions defines the configurable options for rate limiters
// used on reconcilers.
type RateLimiterOptions struct {
	// MinRetryDelay represents the minimum amount of time in which an
	// object being reconciled will have to wait before a retry.
	MinRetryDelay time.Duration

	// MaxRetryDelay represents the maximum amount of time in which an
	// object being reconciled will have to wait before a retry.
	MaxRetryDelay time.Duration
}

// BindFlags will parse the given pflag.FlagSet for the controller and
// set the RateLimiterOptions accordingly.
func (o *RateLimiterOptions) BindFlags(fs *pflag.FlagSet) {
	fs.DurationVar(&o.MinRetryDelay, flagMinRetryDelay, defaultMinRetryDelay,
		"The minimum amount of time for which an object being reconciled will have to wait before a retry.")
	fs.DurationVar(&o.MaxRetryDelay, flagMaxRetryDelay, defaultMaxRetryDelay,
		"The maximum amount of time for which an object being reconciled will have to wait before a retry.")
}

// GetRateLimiter returns a new exponential failure ratelimiter.RateLimiter
// based on RateLimiterOptions.
func GetRateLimiter(opts RateLimiterOptions) ratelimiter.RateLimiter {
	return workqueue.NewItemExponentialFailureRateLimiter(
		opts.MinRetryDelay,
		opts.MaxRetryDelay)
}
