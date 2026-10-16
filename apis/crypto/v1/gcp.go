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

import (
	"regexp"
)

// Service provider name.
const (
	ProviderGCP = "gcp"
)

// Our own GCP-related annotations.
const (
	// AnnotationGCPWorkloadIdentityProvider specifies the GCP Workload Identity Provider
	// for the ServiceAccount in the following format:
	//
	//   `^projects/\d{1,30}/locations/global/workloadIdentityPools/[^/]{1,100}/providers/[^/]{1,100}$`
	AnnotationGCPWorkloadIdentityProvider GCPWorkloadIdentityProvider = GroupAuthGCP + "/workload-identity-provider"
)

// GCP-borrowed annotations.
const (
	// AnnotationGCPServiceAccount specifies the GCP Service Account email for the
	// ServiceAccount in the following format:
	//
	//   `^[a-zA-Z0-9-]{1,100}@[a-zA-Z0-9-]{1,100}\.iam\.gserviceaccount\.com$`
	AnnotationGCPServiceAccount GCPServiceAccount = GroupGKEIAM + "/gcp-service-account"
)

// Patterns.
const (
	// ClusterPatternGCP is the regex pattern for validating GCP cluster resource names.
	ClusterPatternGCP GCPCluster = `^projects/[^/]{1,200}/locations/[^/]{1,200}/clusters/[^/]{1,200}$`
)

var gcpWorkloadIdentityProviderRegex = regexp.MustCompile(
	`^projects/\d{1,30}/locations/global/workloadIdentityPools/[^/]{1,100}/providers/[^/]{1,100}$`)

// GCPWorkloadIdentityProvider is the type for the GCP Workload Identity Provider annotation.
type GCPWorkloadIdentityProvider string

// String implements the fmt.Stringer interface.
func (GCPWorkloadIdentityProvider) String() string {
	return string(AnnotationGCPWorkloadIdentityProvider)
}

// Extract checks if the GCPWorkloadIdentityProvider is in the correct format and returns it.
func (GCPWorkloadIdentityProvider) Extract(saAnnotations map[string]string) (string, bool, error) {
	const key = string(AnnotationGCPWorkloadIdentityProvider)
	regex := gcpWorkloadIdentityProviderRegex
	return extractServiceAccountAnnotation(key, saAnnotations, regex)
}

var gcpServiceAccountEmailRegex = regexp.MustCompile(
	`^[a-zA-Z0-9-]{1,100}@[a-zA-Z0-9-]{1,100}\.iam\.gserviceaccount\.com$`)

// GCPServiceAccount is the type for the GCP Service Account email annotation.
type GCPServiceAccount string

// String implements the fmt.Stringer interface.
func (GCPServiceAccount) String() string {
	return string(AnnotationGCPServiceAccount)
}

// Extract checks if the GCPServiceAccount is in the correct format and returns it.
func (GCPServiceAccount) Extract(saAnnotations map[string]string) (string, bool, error) {
	const key = string(AnnotationGCPServiceAccount)
	regex := gcpServiceAccountEmailRegex
	return extractServiceAccountAnnotation(key, saAnnotations, regex)
}

var gcpClusterPatternRegex = regexp.MustCompile(ClusterPatternGCP.String())

// GCPCluster is the type for the GCP cluster resource name pattern.
type GCPCluster string

// String implements the fmt.Stringer interface.
func (GCPCluster) String() string {
	return string(ClusterPatternGCP)
}

// Validate checks if the GCPCluster is in the correct format.
func (GCPCluster) Validate(cluster string) error {
	_, _, err := parseKubeConfigCluster(cluster, gcpClusterPatternRegex)
	if err != nil {
		return err
	}
	return nil
}
