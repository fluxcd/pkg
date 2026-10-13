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

// Top-level group.
const (
	// GroupCore is the core API group for Flux.
	GroupCore = "fluxcd.io"
)

// Level 1 groups.
const (
	// GroupAuth is the API group for authentication-related annotations.
	GroupAuth = "auth." + GroupCore

	// GroupMTLS is the API group for mTLS-related annotations.
	GroupMTLS = "mtls." + GroupCore
)

// Level 2 groups (auth).
const (
	// GroupAuthCredential is the API group for credential-related annotations.
	GroupAuthCredential = "credential." + GroupAuth

	// GroupAuthTransitEngine is the API group for transit engine-related annotations.
	GroupAuthTransitEngine = "transit-engine." + GroupAuth

	// GroupAuthAWS is the API group for AWS-related authentication annotations.
	GroupAuthAWS = "aws." + GroupAuth

	// GroupAuthGCP is the API group for GCP-related authentication annotations.
	GroupAuthGCP = "gcp." + GroupAuth

	// GroupAuthDockerHub is the API group for Docker Hub-related authentication annotations.
	GroupAuthDockerHub = "dockerhub." + GroupAuth
)

// Level 2 groups (mtls).
const (
	// GroupMTLSSPIFFE is the API group for SPIFFE-related mTLS annotations.
	GroupMTLSSPIFFE = "spiffe." + GroupMTLS
)

// Groups borrowed from cloud providers for UX consistency with the managed Kubernetes services.
const (
	// GroupAWSEKS is the API group for AWS EKS-related annotations.
	GroupAWSEKS = "eks.amazonaws.com"

	// GroupAzureWorkloadIdentity is the API group for Azure Workload Identity-related annotations.
	GroupAzureWorkloadIdentity = "azure.workload.identity"

	// GroupGKEIAM is the API group for GKE IAM-related annotations.
	GroupGKEIAM = "iam.gke.io"
)
