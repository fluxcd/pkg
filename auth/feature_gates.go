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

// FeatureGateObjectLevelWorkloadIdentity is a feature gate that enables the use of
// object-level workload identity for authentication. Controllers register this
// feature gate and are responsible for evaluating it.
const FeatureGateObjectLevelWorkloadIdentity = "ObjectLevelWorkloadIdentity"

// FeatureGateObjectLevelWorkloadIdentityRetirementReason provides the reason why the
// ObjectLevelWorkloadIdentity feature gate is retired.
const FeatureGateObjectLevelWorkloadIdentityRetirementReason = "the feature is now always enabled. " +
	"make sure to grant RBAC for the controller to create ServiceAccount tokens if using the feature."
