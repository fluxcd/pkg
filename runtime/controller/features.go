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

package controller

const (
	// FeatureGateAdditiveCELDependencyCheck controls whether the CEL
	// dependency check should be additive, meaning that the built-in readiness
	// check will be added to the user-defined CEL expressions.
	FeatureGateAdditiveCELDependencyCheck = "AdditiveCELDependencyCheck"

	// FeatureGateCacheSecretsAndConfigMaps controls whether Secrets
	// and ConfigMaps should be cached.
	// When enabled, it will cache both object types, resulting in increased
	// memory usage and cluster-wide RBAC permissions (list and watch).
	FeatureGateCacheSecretsAndConfigMaps = "CacheSecretsAndConfigMaps"

	// FeatureGateExternalArtifact controls whether the
	// ExternalArtifact source type is enabled.
	FeatureGateExternalArtifact = "ExternalArtifact"

	// FeatureGateDisableConfigWatchers controls whether the
	// watching of ConfigMaps and Secrets is disabled.
	FeatureGateDisableConfigWatchers = "DisableConfigWatchers"

	// FeatureGateDirectSourceFetch controls whether the
	// source objects are fetched directly from the API Server
	// instead of relying on controller-runtime's cache.
	// Use with caution, as it may have performance implications on large
	// clusters with many objects or with high reconciliation rates.
	FeatureGateDirectSourceFetch = "DirectSourceFetch"
)
