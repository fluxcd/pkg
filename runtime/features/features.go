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

package features

import (
	"fmt"

	"github.com/go-logr/logr"
	"github.com/spf13/pflag"
	cliflag "k8s.io/component-base/cli/flag"
)

const (
	flagFeatureGates = "feature-gates"
)

var featureGates map[string]bool
var loaded bool

// FeatureGates is a helper to manage feature switches.
//
// Controllers can set their supported features and then at runtime
// verify which ones are enabled/disabled.
//
// Callers have to call BindFlags, and then call SupportedFeatures to
// set the supported features and their default values.
type FeatureGates struct {
	log         *logr.Logger
	cliFeatures map[string]bool
}

// WithLogger sets the logger to be used when loading supported features.
func (o *FeatureGates) WithLogger(l logr.Logger) *FeatureGates {
	o.log = &l
	return o
}

// SupportedFeatures sets the supported features and their default values.
func (o *FeatureGates) SupportedFeatures(features map[string]bool) error {
	loaded = true
	featureGates = features

	for k, v := range o.cliFeatures {
		if _, ok := featureGates[k]; ok {
			featureGates[k] = v
		} else {
			return fmt.Errorf("feature-gate '%s' not supported", k)
		}
		if o.log != nil {
			o.log.Info("loading feature gate", k, v)
		}
	}
	return nil
}

// Enabled verifies whether the feature is enabled or not.
func Enabled(feature string) (bool, error) {
	if !loaded {
		return false, fmt.Errorf("supported features not set")
	}
	if enabled, ok := featureGates[feature]; ok {
		return enabled, nil
	}
	return false, fmt.Errorf("feature-gate '%s' not supported", feature)
}

// BindFlags will parse the given pflag.FlagSet and load feature gates accordingly.
func (o *FeatureGates) BindFlags(fs *pflag.FlagSet) {
	fs.Var(cliflag.NewMapStringBool(&o.cliFeatures), flagFeatureGates,
		"A comma separated list of key=value pairs defining the state of experimental features.")
}
