//go:build integration
// +build integration

/*
Copyright 2022 The Flux authors

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

package integration

import (
	"context"
	"fmt"

	tfjson "github.com/hashicorp/terraform-json"

	"github.com/fluxcd/test-infra/tftestenv"
)

const (
	// gcpIAMAnnotation is the key for the annotation on the kubernetes serviceaccount
	// with the email address of the IAM service account on GCP.
	gcpIAMAnnotation = "iam.gke.io/gcp-service-account"

	// gcpWorkloadIdentityProviderAnnotation is the key for the annotation on the kubernetes serviceaccount
	// with the name of the workload identity provider on GCP.
	gcpWorkloadIdentityProviderAnnotation = "gcp.auth.fluxcd.io/workload-identity-provider"
)

// createKubeconfigGKE constructs kubeconfig from the terraform state output at
// the given kubeconfig path.
func createKubeconfigGKE(ctx context.Context, state map[string]*tfjson.StateOutput, kcPath string) error {
	kubeconfigYaml, ok := state["gcp_kubeconfig"].Value.(string)
	if !ok || kubeconfigYaml == "" {
		return fmt.Errorf("failed to obtain kubeconfig from tf output")
	}
	return tftestenv.CreateKubeconfigGKE(ctx, kubeconfigYaml, kcPath)
}

// registryLoginGAR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
func registryLoginGAR(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// NOTE: GAR accepts dynamic repository creation by just pushing a new image
	// with a new repository name.
	testRepos := map[string]string{}

	project := output["gcp_project"].Value.(string)
	region := output["gcp_region"].Value.(string)
	repositoryID := output["gcp_artifact_repository"].Value.(string)
	artifactRegistryURL, artifactRepoURL := tftestenv.GetGoogleArtifactRegistryAndRepository(project, region, repositoryID)
	if err := tftestenv.RegistryLoginGCR(ctx, artifactRegistryURL); err != nil {
		return nil, err
	}
	testRepos["artifact_registry"] = artifactRepoURL + "/" + randStringRunes(5)

	return testRepos, nil
}

// pushAppTestImagesGAR pushes test app images that are being tested. It must be
// called after registryLoginGAR to ensure the local docker client is already
// logged in and is capable of pushing the test images.
func pushAppTestImagesGAR(ctx context.Context, localImgs map[string]string, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	project := output["gcp_project"].Value.(string)
	region := output["gcp_region"].Value.(string)
	repositoryID := output["gcp_artifact_repository"].Value.(string)
	return tftestenv.PushTestAppImagesGCR(ctx, localImgs, project, region, repositoryID)
}

// getWISAAnnotationsGCP returns workload identity annotations for a kubernetes ServiceAccount
func getWISAAnnotationsGCP(output map[string]*tfjson.StateOutput) (map[string]string, error) {
	saEmail := output["wi_iam_serviceaccount_email"].Value.(string)
	if saEmail == "" {
		return nil, fmt.Errorf("no GCP serviceaccount email in terraform output")
	}

	return map[string]string{
		gcpIAMAnnotation: saEmail,
	}, nil
}

// getWIFederationSAAnnotationsGCP returns workload identity federation annotations for a kubernetes ServiceAccount
func getWIFederationSAAnnotationsGCP(output map[string]*tfjson.StateOutput) (map[string]string, error) {
	workloadIdentityProvider := output["workload_identity_provider"].Value.(string)
	if workloadIdentityProvider == "" {
		return nil, fmt.Errorf("no GCP workload identity provider in terraform output")
	}

	return map[string]string{
		gcpWorkloadIdentityProviderAnnotation: workloadIdentityProvider,
	}, nil
}

// getClusterResourceGCP returns the cluster resource for kubeconfig auth tests.
func getClusterResourceGCP(output map[string]*tfjson.StateOutput) (string, error) {
	clusterResource := output["cluster_resource"].Value.(string)
	if clusterResource == "" {
		return "", fmt.Errorf("no GKE cluster id in terraform output")
	}
	return clusterResource, nil
}

// getClusterAddressGCP returns the cluster address for kubeconfig auth tests.
func getClusterAddressGCP(output map[string]*tfjson.StateOutput) (string, error) {
	clusterAddress := output["cluster_endpoint"].Value.(string)
	if clusterAddress == "" {
		return "", fmt.Errorf("no GKE cluster address in terraform output")
	}
	return clusterAddress, nil
}

// getClusterUsersGCP returns the cluster users for kubeconfig auth tests.
func getClusterUsersGCP(output map[string]*tfjson.StateOutput) ([]string, error) {
	var clusterUsers []string

	for _, key := range []string{
		"wi_iam_serviceaccount_email",
		"wi_k8s_sa_principal_direct_access",
		"wi_k8s_sa_principal_direct_access_federation",
	} {
		if clusterUser := output[key].Value.(string); clusterUser != "" {
			clusterUsers = append(clusterUsers, clusterUser)
		}
	}

	return clusterUsers, nil
}

// When implemented, getGitTestConfigGCP would return the git-specific test config for GCP
func getGitTestConfigGCP(outputs map[string]*tfjson.StateOutput) (*gitTestConfig, error) {
	return nil, fmt.Errorf("NotImplemented for GCP")
}

// When implemented, grantPermissionsToGitRepositoryGCP would grant the required permissions to Google cloud source repositories
func grantPermissionsToGitRepositoryGCP(ctx context.Context, cfg *gitTestConfig, output map[string]*tfjson.StateOutput) error {
	return fmt.Errorf("NotImplemented for GCP")
}

// When implemented, revokePermissionsToGitRepositoryGCP would revoke the permissions granted to Google cloud source repositories
func revokePermissionsToGitRepositoryGCP(ctx context.Context, cfg *gitTestConfig, outputs map[string]*tfjson.StateOutput) error {
	return fmt.Errorf("NotImplemented for GCP")
}
