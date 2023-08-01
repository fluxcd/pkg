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

package jitter

import (
	"errors"
	"github.com/spf13/pflag"
	"math/rand"
	"sync"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	flagIntervalJitter              = "interval-jitter-percentage"
	defaultIntervalJitterPercentage = 10
)

var (
	globalIntervalJitter     Duration = NoJitter
	globalIntervalJitterOnce sync.Once

	errInvalidIntervalJitter = errors.New("the interval jitter percentage must be a non-negative value and less than 100")
)

// SetGlobalIntervalJitter sets the global interval jitter. It is safe to call
// this method multiple times, but only the first call will have an effect.
func SetGlobalIntervalJitter(p float64, rand *rand.Rand) {
	globalIntervalJitterOnce.Do(func() {
		globalIntervalJitter = Percent(p, rand)
	})
}

// RequeueAfterResult returns a result with a requeue-after interval that has
// been jittered. It will not modify the result if it is zero or is marked
// to requeue immediately.
//
// To use this function, you must first initialize the global jitter with
// SetGlobalIntervalJitter.
func RequeueAfterResult(res ctrl.Result) ctrl.Result {
	if res.IsZero() || res.Requeue == true {
		return res
	}
	if after := res.RequeueAfter; after > 0 {
		res.RequeueAfter = globalIntervalJitter(after)
	}
	return res
}

// IntervalDuration returns a jittered duration based on the given interval.
//
// To use this function, you must first initialize the global jitter with
// SetGlobalIntervalJitter.
func IntervalDuration(d time.Duration) time.Duration {
	return globalIntervalJitter(d)
}

// Interval is used to configure the interval jitter for a controller using
// command line flags. To use it, create an Interval and call BindFlags, then
// call SetGlobalJitter with a rand.Rand (or nil to use the default).
//
// Applying jitter to the interval duration can be useful to mitigate spikes in
// memory and CPU usage caused by many resources being configured with the same
// interval.
//
// When 1000 resources are configured to requeue every 5 minutes with a
// concurrency setting of 50 and a process time of approximately 1 second per
// resource.
//
// Without jitter, all 1000 resources will requeue every 5 minutes, resulting
// in 50 resources requeueing simultaneously every second over a 20-second
// window.
//
// However, when we apply +/-10% jitter to the interval duration, the requeueing
// will be spread out over a 1-minute window. As a result, the number of
// resources requeueing per second will vary between approximately 15 to 18.33.
//
// This smoother workload distribution can result in significant reductions in
// the impact of CPU and memory spikes. This improvement in workload
// distribution also translates into benefits for the Go garbage collector.
// Notably, the garbage collector experiences reduced GC bursts and more
// frequent collections, leading to improved overall performance.
type Interval struct {
	// Percentage of jitter to apply to interval durations. A value of 10
	// will apply a jitter of +/-10% to the interval duration. It can not be negative,
	// and must be less than 100.
	Percentage uint8
}

// BindFlags will parse the given pflag.FlagSet and load the interval jitter
// with the default value of 10%.
func (o *Interval) BindFlags(fs *pflag.FlagSet) {
	o.BindFlagsWithDefault(fs, -1)
}

// BindFlagsWithDefault will parse the given pflag.FlagSet and load the interval
// jitter. The defaultPercentage is used to set the default value for the
// interval jitter percentage. If the defaultPercentage is negative, then the
// default value (of 10%) will be used.
func (o *Interval) BindFlagsWithDefault(fs *pflag.FlagSet, defaultPercentage int) {
	if defaultPercentage < 0 {
		defaultPercentage = defaultIntervalJitterPercentage
	}
	fs.Uint8Var(&o.Percentage, flagIntervalJitter, uint8(defaultPercentage),
		"Percentage of jitter to apply to interval durations. A value of 10 "+
			"will apply a jitter of +/-10% to the interval duration. It cannot be "+
			"negative, and must be less than 100.")
}

// SetGlobalJitter sets the global interval jitter. It is safe to call this
// method multiple times, but only the first call will have an effect.
func (o *Interval) SetGlobalJitter(rand *rand.Rand) error {
	if o.Percentage >= 100 {
		return errInvalidIntervalJitter
	}
	if o.Percentage > 0 && o.Percentage < 100 {
		SetGlobalIntervalJitter(float64(o.Percentage)/100.0, rand)
	}
	return nil
}
