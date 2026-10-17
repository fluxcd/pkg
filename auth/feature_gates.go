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

// FeatureGateObjectLevelWorkloadIdentity is a feature gate that enables the use of
// object-level workload identity for authentication. Controllers register this
// feature gate and are responsible for evaluating it.
const FeatureGateObjectLevelWorkloadIdentity = "ObjectLevelWorkloadIdentity"

// ErrObjectLevelWorkloadIdentityNotEnabled is returned when object-level
// workload identity is attempted but not enabled.
var ErrObjectLevelWorkloadIdentityNotEnabled = fmt.Errorf(
	"%s feature gate is not enabled", FeatureGateObjectLevelWorkloadIdentity)
