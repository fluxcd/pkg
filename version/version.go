/*
Copyright 2020 The Flux authors

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

package version

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// ParseVersion parses a version string and returns a semver.Version object.
// The validation is looser than the official semver spec, allowing for
// a 'v' prefix and 0-prefixed numbers in the major, minor, and patch segments
// (e.g., v2025.02.03-rc.1 is considered valid).
func ParseVersion(v string) (*semver.Version, error) {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil, semver.ErrInvalidSemVer
	}

	return semver.NewVersion(v)
}

// Sort filters the given strings based on the provided semver range
// and sorts them in descending order.
func Sort(c *semver.Constraints, vs []string) []string {
	var versions []*semver.Version
	for _, v := range vs {
		if pv, err := ParseVersion(v); err == nil && (c == nil || c.Check(pv)) {
			versions = append(versions, pv)
		}
	}
	sort.Sort(sort.Reverse(semver.Collection(versions)))
	sorted := make([]string, 0, len(versions))
	for _, v := range versions {
		sorted = append(sorted, v.Original())
	}
	return sorted
}

// controllerVersion holds a major and minor version for a controller.
type controllerVersion struct {
	major int
	minor int
}

// BaselineFlux2Minor is the baseline minor version of the flux2
// distribution used for computing the minor versions of the
// controllers in that version. This is Flux v2.7 at the moment.
// When a new controller is added to the distribution, this
// baseline MUST be updated, as our release workflows depend
// on it. Flux v2.7 is, at the moment, the latest version where
// a controller was added to the distribution: source-watcher.
const BaselineFlux2Minor = 7

// baselineControllerMinors are the minor versions for
// the controllers in the baseline version of the flux2
// distribution, i.e. Flux v2.{baselineFlux2Minor}.
var baselineControllerMinors = map[string]controllerVersion{
	// The distribution itself.
	"flux2": {2, BaselineFlux2Minor},

	// Controllers.
	"source-controller":           {1, 7},
	"kustomize-controller":        {1, 7},
	"helm-controller":             {1, 4},
	"notification-controller":     {1, 7},
	"image-reflector-controller":  {1, 0},
	"image-automation-controller": {1, 0},
	"source-watcher":              {2, 0},
}

// ControllerMajor returns the major version for the given controller name.
func ControllerMajor(controllerName string) (int, error) {
	cv, ok := baselineControllerMinors[controllerName]
	if !ok {
		return 0, fmt.Errorf("unknown controller %q: not in baseline mapping", controllerName)
	}
	return cv.major, nil
}

// ControllerMinorForFluxMinor computes the controller minor
// version for the given flux2 distribution minor version.
// This function also supports controllerName="flux2".
func ControllerMinorForFluxMinor(controllerName string, flux2Minor int) (int, error) {
	baselineControllerMinor, ok := baselineControllerMinors[controllerName]
	if !ok {
		return 0, fmt.Errorf("unknown controller %q: not in baseline mapping", controllerName)
	}
	return flux2Minor - BaselineFlux2Minor + baselineControllerMinor.minor, nil
}

// FluxMinorForControllerMinor computes the flux2 distribution
// minor version for the given controller minor version.
// This function also supports controllerName="flux2" and
// controllerMinor=<any minor version of flux2>.
func FluxMinorForControllerMinor(controllerName string, controllerMinor int) (int, error) {
	baselineControllerMinor, ok := baselineControllerMinors[controllerName]
	if !ok {
		return 0, fmt.Errorf("unknown controller %q: not in baseline mapping", controllerName)
	}
	return controllerMinor - baselineControllerMinor.minor + BaselineFlux2Minor, nil
}
