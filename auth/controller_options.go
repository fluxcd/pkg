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
)

// ErrInconsistentObjectLevelConfiguration is used when the controller has
// an inconsistent object-level workload identity configuration.
var ErrInconsistentObjectLevelConfiguration = fmt.Errorf(
	"cannot set default service accounts when the feature gate %s is not enabled",
	FeatureGateObjectLevelWorkloadIdentity)

// InconsistentObjectLevelConfiguration checks if the controller's object-level
// workload identity configuration is inconsistent.
func InconsistentObjectLevelConfiguration() bool {
	return !IsObjectLevelWorkloadIdentityEnabled() &&
		(GetDefaultServiceAccount() != "" ||
			GetDefaultKubeConfigServiceAccount() != "" ||
			GetDefaultDecryptionServiceAccount() != "")
}
