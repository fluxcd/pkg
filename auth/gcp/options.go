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

package gcp

import (
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"
)

const serviceAccountEmailPattern = `^[a-zA-Z0-9-]{1,100}@[a-zA-Z0-9-]{1,100}\.iam\.gserviceaccount\.com$`

var serviceAccountEmailRegex = regexp.MustCompile(serviceAccountEmailPattern)

func getServiceAccountEmail(serviceAccount corev1.ServiceAccount) (string, error) {
	const key = "iam.gke.io/gcp-service-account"
	email := serviceAccount.Annotations[key]
	if email == "" {
		return "", nil
	}
	if !serviceAccountEmailRegex.MatchString(email) {
		return "", fmt.Errorf("invalid %s annotation: '%s'. must match %s",
			key, email, serviceAccountEmailPattern)
	}
	return email, nil
}

const workloadIdentityProviderPattern = `^projects/\d{1,30}/locations/global/workloadIdentityPools/[^/]{1,100}/providers/[^/]{1,100}$`

var workloadIdentityProviderRegex = regexp.MustCompile(workloadIdentityProviderPattern)

func getWorkloadIdentityProviderAudience(serviceAccount corev1.ServiceAccount) (string, error) {
	const key = "gcp.auth.fluxcd.io/workload-identity-provider"
	wip := serviceAccount.Annotations[key]
	if wip == "" {
		return "", nil
	}
	if !workloadIdentityProviderRegex.MatchString(wip) {
		return "", fmt.Errorf("invalid %s annotation: '%s'. must match %s",
			key, wip, workloadIdentityProviderPattern)
	}
	return fmt.Sprintf("//iam.googleapis.com/%s", wip), nil
}
