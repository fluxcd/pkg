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
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
)

// GetRegistryFromArtifactRepository returns the registry host of the given
// artifact repository, i.e. of an OCI repository URL or an image reference.
func GetRegistryFromArtifactRepository(artifactRepository string) (string, error) {
	registry := strings.TrimSuffix(artifactRepository, "/")
	if strings.ContainsRune(registry, '/') {
		ref, err := name.ParseReference(registry)
		if err != nil {
			return "", fmt.Errorf("failed to parse artifact repository '%s': %w",
				artifactRepository, err)
		}
		return ref.Context().RegistryStr(), nil
	}
	return registry, nil
}

// ParseArtifactRepository validates the given artifact repository against the
// rules of the given service provider and returns the provider-specific input
// required to issue credentials for it. The input is also used as part of the
// credentials cache key.
//
// For the `aws` service provider the input is the ECR region, unless the
// registry is the public ECR, in which case it is RegistryAWSPublicECR. For the
// `gcp` service provider the input is the service provider name, as GCP
// credentials are not bound to the registry. For the `azure` service provider
// the input is the ACR registry host.
func ParseArtifactRepository(serviceProvider, artifactRepository string) (string, error) {
	registry, err := GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		return "", err
	}

	switch serviceProvider {
	case ProviderAWS:
		// Both public hostnames are normalized to RegistryAWSPublicECR, which
		// is what selects the public authorization token API and the us-east-1
		// region downstream.
		if registry == RegistryAWSPublicECR || registry == RegistryAWSPublicECRDualStack {
			return RegistryAWSPublicECR, nil
		}
		matches := registryAWSRegexp.FindStringSubmatch(registry)
		if matches == nil {
			return "", fmt.Errorf("invalid AWS registry: '%s'. must match %s",
				registry, RegistryPatternAWS)
		}
		return matches[2], nil
	case ProviderGCP:
		if !registryGCPRegexp.MatchString(registry) {
			return "", fmt.Errorf("invalid GCP registry: '%s'. must match %s",
				registry, RegistryPatternGCP)
		}
		return ProviderGCP, nil
	case ProviderAzure:
		if !registryAzureRegexp.MatchString(registry) {
			return "", fmt.Errorf("invalid Azure registry: '%s'. must match %s",
				registry, RegistryPatternAzure)
		}
		return registry, nil
	}
	return "", fmt.Errorf("service provider '%s' does not support artifact registries", serviceProvider)
}

// GetAWSRegionFromArtifactRegistryInput returns the AWS region for the given
// artifact registry input returned by ParseArtifactRepository. The public ECR
// registry always uses the us-east-1 region.
//
// https://docs.aws.amazon.com/AmazonECR/latest/public/public-registry-auth.html#public-registry-auth-token
func GetAWSRegionFromArtifactRegistryInput(registryInput string) string {
	if registryInput == RegistryAWSPublicECR {
		return "us-east-1"
	}
	return registryInput
}
