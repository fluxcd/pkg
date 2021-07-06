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

package leaderelection

import (
	"time"

	"github.com/spf13/pflag"
)

const (
	flagEnable          = "enable-leader-election"
	flagReleaseOnCancel = "leader-election-release-on-cancel"
	flagLeaseDuration   = "leader-election-lease-duration"
	flagRenewDeadline   = "leader-election-renew-deadline"
	flagRetryPeriod     = "leader-election-retry-period"
)

// Options contains the runtime configuration for leader election.
//
// The struct can be used in the main.go file of your controller by binding it to the main flag set, and then utilizing
// the configured options later:
//
//	func main() {
//		var (
//			// other controller specific configuration variables
//			leaderElectionOptions leaderelection.Options
//		)
//
//		// Bind the options to the main flag set, and parse it
//		leaderElectionOptions.BindFlags(flag.CommandLine)
//		flag.Parse()
//
//		// Use the values during the initialisation of the manager
//		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
//			...other options
//			LeaderElection:                leaderElectionOptions.Enable,
//			LeaderElectionReleaseOnCancel: leaderElectionOptions.ReleaseOnCancel,
//			LeaseDuration:                 &leaderElectionOptions.LeaseDuration,
//			RenewDeadline:                 &leaderElectionOptions.RenewDeadline,
//			RetryPeriod:                   &leaderElectionOptions.RetryPeriod,
//		})
//	}
type Options struct {
	// Enable determines whether or not to use leader election when starting the manager.
	Enable bool

	// ReleaseOnCancel defines if the leader should step down voluntarily when the Manager ends. This requires the
	// binary to immediately end when the Manager is stopped, otherwise this setting is unsafe. Setting this
	// significantly speeds up voluntary leader transitions as the new leader doesn't have to wait LeaseDuration time
	// first.
	ReleaseOnCancel bool

	// LeaseDuration is the duration that non-leader candidates will wait to force acquire leadership. This is measured
	// against time of last observed ack. Default is 35 seconds.
	LeaseDuration time.Duration

	// RenewDeadline is the duration that the acting controlplane will retry refreshing leadership before giving up.
	// Default is 30 seconds.
	RenewDeadline time.Duration

	// RetryPeriod is the duration the LeaderElector clients should wait between tries of actions. Default is 5 seconds.
	RetryPeriod time.Duration
}

// BindFlags will parse the given pflag.FlagSet for leader election option flags and set the Options accordingly.
func (o *Options) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.Enable, flagEnable, false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.BoolVar(&o.ReleaseOnCancel, flagReleaseOnCancel, true,
		"Defines if the leader should step down voluntarily on controller manager shutdown.")
	fs.DurationVar(&o.LeaseDuration, flagLeaseDuration, 35*time.Second,
		"Interval at which non-leader candidates will wait to force acquire leadership (duration string).")
	fs.DurationVar(&o.RenewDeadline, flagRenewDeadline, 30*time.Second,
		"Duration that the leading controller manager will retry refreshing leadership before giving up (duration string).")
	fs.DurationVar(&o.RetryPeriod, flagRetryPeriod, 5*time.Second,
		"Duration the LeaderElector clients should wait between tries of actions (duration string).")
}
