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

package v1

import "errors"

// Keys read from a .spec.kubeConfig.configMapRef's ConfigMap.
const (
	// KubeConfigProvider is the required Kubernetes service provider.
	// One of `aws`, `azure`, `gcp` or `generic`.
	KubeConfigProvider = "provider"

	// KubeConfigServiceAccountName is the optional name of the Kubernetes ServiceAccount
	// in the same namespace of the ConfigMap that should be used for authentication. If
	// not specified, the controller ServiceAccount will be used.
	KubeConfigServiceAccountName = "serviceAccountName"

	// KubeConfigCluster is the fully qualified resource name of the Kubernetes cluster
	// as represent in a cloud provider API. Ignored by the `generic` provider. Required
	// when one of `address` or `ca.crt` is not set.
	KubeConfigCluster = "cluster"

	// KubeConfigAddress is the address of the Kubernetes API server. Required for
	// `generic`. For the other providers, if not specified, the first address in
	// the cluster resource will be used, and if specified, it must match one of
	// the addresses in the cluster resource. If audiences is not set, will be used
	// as the default audience for the `generic` provider.
	KubeConfigAddress = "address"

	// KubeConfigCA is the optional PEM-encoded CA certificate for the Kubernetes API
	// server. If not set, the controller will use the CA certificate from the
	// cluster resource.
	KubeConfigCA = "ca.crt"

	// KubeConfigLegacyAudiences is an optional legacy key holding a list of
	// line-break-separated strings for minting a Kubernetes ServiceAccount token.
	// Defaults to the `address` for the `generic` provider, or to specific values
	// for the other providers depending on the provider.
	//
	// Legacy/forward compatibility prevention rule: this ConfigMap key MUST NOT be
	// allowed when a configured `serviceAccountName` contains any of the new annotations
	// introduced in RFC-0014.
	KubeConfigLegacyAudiences = "audiences"
)

// Errors.
var (
	// ErrClusterRequired is the error used for the requirement of a cluster when
	// either the `address` or the `ca.crt` is not set.
	ErrClusterRequired = errors.New("cluster is required when one of {address, ca.crt} is missing in the kubeconfig ConfigMap")
)
