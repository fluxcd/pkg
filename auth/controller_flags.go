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
)

// ErrDefaultServiceAccountNotFound is returned when a default service account
// configured by the operator is not found in the user's namespace.
var ErrDefaultServiceAccountNotFound = fmt.Errorf(
	"the specified default service account does not exist in the object namespace. " +
		"your cluster is subject to multi-tenant workload identity lockdown, reach out " +
		"to your cluster administrator for help")
