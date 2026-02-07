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

const (
	// ControllerFlagDefaultServiceAccount defines the flag for the default service account name
	// to be used when .spec.serviceAccountName is not specified in the object.
	ControllerFlagDefaultServiceAccount = "default-service-account"

	// ControllerFlagDefaultKubeConfigServiceAccount defines the flag for the default
	// service account name to be used when .data.serviceAccountName is not specified
	// in the ConfigMap referenced by .spec.kubeConfig.configMapRef.
	ControllerFlagDefaultKubeConfigServiceAccount = "default-kubeconfig-service-account"

	// ControllerFlagDefaultDecryptionServiceAccount defines the flag for the default
	// service account name to be used when .spec.decryption.serviceAccountName is
	// not specified in the object.
	ControllerFlagDefaultDecryptionServiceAccount = "default-decryption-service-account"

	// ControllerFlagOCISkipRegistryValidation defines the flag for skipping OCI registry
	// domain validation for cloud provider authentication. This allows using custom
	// registry proxies/gateways with workload identity authentication.
	ControllerFlagOCISkipRegistryValidation = "oci-skip-registry-validation"
)

var (
	// defaultServiceAccount stores the default service account name
	// for workload identity.
	defaultServiceAccount string

	// defaultKubeConfigServiceAccount stores the default kubeconfig
	// service account name.
	defaultKubeConfigServiceAccount string

	// defaultDecryptionServiceAccount stores the default decryption
	// service account name.
	defaultDecryptionServiceAccount string

	// ociSkipRegistryValidation stores whether to skip OCI registry
	// domain validation for cloud provider authentication.
	ociSkipRegistryValidation bool
)

// ErrDefaultServiceAccountNotFound is returned when a default service account
// configured by the operator is not found in the user's namespace.
var ErrDefaultServiceAccountNotFound = fmt.Errorf("the specified default service account does not exist in the object namespace. your cluster is subject to multi-tenant workload identity lockdown, reach out to your cluster administrator for help")

// SetDefaultServiceAccount sets the default service account name for workload identity.
func SetDefaultServiceAccount(sa string) {
	defaultServiceAccount = sa
}

// SetDefaultKubeConfigServiceAccount sets the default kubeconfig service account name.
func SetDefaultKubeConfigServiceAccount(sa string) {
	defaultKubeConfigServiceAccount = sa
}

// SetDefaultDecryptionServiceAccount sets the default decryption service account name.
func SetDefaultDecryptionServiceAccount(sa string) {
	defaultDecryptionServiceAccount = sa
}

// GetDefaultServiceAccount returns the default service account name for workload identity.
func GetDefaultServiceAccount() string {
	return defaultServiceAccount
}

// GetDefaultKubeConfigServiceAccount returns the default kubeconfig service account name.
func GetDefaultKubeConfigServiceAccount() string {
	return defaultKubeConfigServiceAccount
}

// GetDefaultDecryptionServiceAccount returns the default decryption service account name.
func GetDefaultDecryptionServiceAccount() string {
	return defaultDecryptionServiceAccount
}

// SetOCISkipRegistryValidation sets whether to skip OCI registry domain validation.
func SetOCISkipRegistryValidation(skip bool) {
	ociSkipRegistryValidation = skip
}

// GetOCISkipRegistryValidation returns whether to skip OCI registry domain validation.
func GetOCISkipRegistryValidation() bool {
	return ociSkipRegistryValidation
}

func getDefaultServiceAccount() string {
	// Here we can detect a default service account by checking either the default
	// service account or the default kubeconfig service account because these two
	// are supposed to never be set simultaneously. The controller main functions
	// must ensure this property.
	if s := GetDefaultServiceAccount(); s != "" {
		return s
	}
	return GetDefaultKubeConfigServiceAccount()
}
