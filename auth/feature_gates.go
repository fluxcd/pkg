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

package auth

import (
	"fmt"
	"os"
)

// FeatureGateObjectLevelWorkloadIdentity is a feature gate that enables the use of
// object-level workload identity for authentication.
const FeatureGateObjectLevelWorkloadIdentity = "ObjectLevelWorkloadIdentity"

// ErrObjectLevelWorkloadIdentityNotEnabled is returned when object-level
// workload identity is attempted but not enabled.
var ErrObjectLevelWorkloadIdentityNotEnabled = fmt.Errorf(
	"%s feature gate is not enabled", FeatureGateObjectLevelWorkloadIdentity)

// SetFeatureGates sets the default values for the feature gates.
func SetFeatureGates(features map[string]bool) {
	// opt-in from Flux v2.6.
	features[FeatureGateObjectLevelWorkloadIdentity] = false
}

// EnvEnableObjectLevelWorkloadIdentity is the environment variable that
// enables the use of object-level workload identity for authentication.
const EnvEnableObjectLevelWorkloadIdentity = "ENABLE_OBJECT_LEVEL_WORKLOAD_IDENTITY"

// EnableObjectLevelWorkloadIdentity enables the use of object-level workload
// identity for authentication.
func EnableObjectLevelWorkloadIdentity() {
	os.Setenv(EnvEnableObjectLevelWorkloadIdentity, "true")
}

// IsObjectLevelWorkloadIdentityEnabled returns true if the object-level
// workload identity feature gate is enabled.
func IsObjectLevelWorkloadIdentityEnabled() bool {
	return os.Getenv(EnvEnableObjectLevelWorkloadIdentity) == "true"
}
