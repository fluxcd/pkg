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
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/test-infra/tftestenv"
	"github.com/google/uuid"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/graph"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/licensing"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/memberentitlementmanagement"
)

const (
	// azureWIClientIdAnnotation is the key for the client ID annotation on the kubernetes serviceaccount
	azureWIClientIdAnnotation = "azure.workload.identity/client-id"

	// azureWITenantIdAnnotation is the key for the tenant ID annotation on the kubernetes serviceaccount
	azureWITenantIdAnnotation = "azure.workload.identity/tenant-id"
)

// createKubeConfigAKS constructs kubeconfig for an AKS cluster from the
// terraform state output at the given kubeconfig path.
func createKubeConfigAKS(ctx context.Context, state map[string]*tfjson.StateOutput, kcPath string) error {
	kubeconfigYaml, ok := state["aks_kubeconfig"].Value.(string)
	if !ok || kubeconfigYaml == "" {
		return fmt.Errorf("failed to obtain kubeconfig from tf output")
	}
	return tftestenv.CreateKubeconfigAKS(ctx, kubeconfigYaml, kcPath)
}

// registryLoginACR logs into the container/artifact registries using the
// provider's CLI tools and returns a list of test repositories.
func registryLoginACR(ctx context.Context, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// NOTE: ACR registry accept dynamic repository creation by just pushing a
	// new image with a new repository name.
	testRepos := map[string]string{}

	registryURL := output["acr_registry_url"].Value.(string)
	if err := tftestenv.RegistryLoginACR(ctx, registryURL); err != nil {
		return nil, err
	}
	testRepos["acr"] = registryURL + "/" + randStringRunes(5)

	return testRepos, nil
}

// pushAppTestImagesACR pushes test app images that are being tested. It must be
// called after registryLoginACR to ensure the local docker client is already
// logged in and is capable of pushing the test images.
func pushAppTestImagesACR(ctx context.Context, localImgs map[string]string, output map[string]*tfjson.StateOutput) (map[string]string, error) {
	// Get the registry name and construct the image names accordingly.
	registryURL := output["acr_registry_url"].Value.(string)
	return tftestenv.PushTestAppImagesACR(ctx, localImgs, registryURL)
}

// getWISAAnnotationsAzure returns azure workload identity's annotations for
// kubernetes service account using output from terraform.
// https://learn.microsoft.com/en-us/azure/aks/workload-identity-overview?tabs=dotnet#pod-annotations
func getWISAAnnotationsAzure(output map[string]*tfjson.StateOutput) (map[string]string, error) {
	clientID := output["workload_identity_client_id"].Value.(string)
	if clientID == "" {
		return nil, fmt.Errorf("no Azure client id in terraform output")
	}

	tenantID := output["workload_identity_tenant_id"].Value.(string)
	if tenantID == "" {
		return nil, fmt.Errorf("no Azure tenant id in terraform output")
	}

	return map[string]string{
		azureWIClientIdAnnotation: clientID,
		azureWITenantIdAnnotation: tenantID,
	}, nil
}

// Give managed identity permissions on the azure devops project. Refer
// https://learn.microsoft.com/en-us/rest/api/azure/devops/memberentitlementmanagement/service-principal-entitlements/add?view=azure-devops-rest-7.1&tabs=HTTP.
// This can be moved to terraform if/when this PR completes -
// https://github.com/microsoft/terraform-provider-azuredevops/pull/1028
// Returns a string representing the uuid of the entity that was granted permissions
func grantPermissionsToGitRepositoryAzure(ctx context.Context, cfg *gitTestConfig, outputs map[string]*tfjson.StateOutput) error {
	projectId := outputs["azure_devops_project_id"].Value.(string)
	wiObjectId := outputs["workload_identity_object_id"].Value.(string)
	var servicePrincipalID string

	// Create a connection to the organization and create a new client
	connection := azuredevops.NewPatConnection(fmt.Sprintf("https://dev.azure.com/%s", cfg.organization), cfg.gitPat)
	client, err := memberentitlementmanagement.NewClient(ctx, connection)
	if err != nil {
		return err
	}

	uuid, err := uuid.Parse(projectId)
	if err != nil {
		return err
	}
	origin := "AAD"
	kind := "servicePrincipal"
	servicePrincipal := memberentitlementmanagement.ServicePrincipalEntitlement{
		AccessLevel: &licensing.AccessLevel{
			AccountLicenseType: &licensing.AccountLicenseTypeValues.Express,
		},
		ProjectEntitlements: &[]memberentitlementmanagement.ProjectEntitlement{
			{
				ProjectRef: &memberentitlementmanagement.ProjectRef{
					Id: &uuid,
				},
				Group: &memberentitlementmanagement.Group{
					GroupType: &memberentitlementmanagement.GroupTypeValues.ProjectContributor,
				},
			},
		},
		ServicePrincipal: &graph.GraphServicePrincipal{
			Origin:      &origin,
			OriginId:    &wiObjectId,
			SubjectKind: &kind,
		},
	}

	// First request to add new user fails, second request succeeds, add a retry
	retryAttempts := 2
	retryDelay := 1 * time.Second // 1 seconds delay
	attempts := 0
	for attempts < retryAttempts {
		attempts++
		responseValue, err := client.AddServicePrincipalEntitlement(ctx, memberentitlementmanagement.AddServicePrincipalEntitlementArgs{ServicePrincipalEntitlement: &servicePrincipal})
		if err != nil {
			return err
		}

		if !*responseValue.OperationResult.IsSuccess {
			errMsg := getServicePrincipalEntitlementAPIErrorMessage(*responseValue.OperationResult)
			if strings.Contains(errMsg, "VS403283: Could not add user") {
				log.Println("Retryable error encountered", errMsg)
				time.Sleep(retryDelay)
				continue
			} else {
				return errors.New(errMsg)
			}
		}
		uuid := responseValue.OperationResult.ServicePrincipalId
		servicePrincipalID = uuid.String()
		break
	}

	cfg.permissionID = servicePrincipalID
	log.Println("Added service principal entitlement!")

	return nil
}

func getServicePrincipalEntitlementAPIErrorMessage(operationResult memberentitlementmanagement.ServicePrincipalEntitlementOperationResult) string {
	errMsg := "Unknown API error"
	if operationResult.Errors != nil && len(*operationResult.Errors) > 0 {
		var errorMessages []string
		for _, err := range *operationResult.Errors {
			errorMessages = append(errorMessages, fmt.Sprintf("(%v) %s", *err.Key, *err.Value))
		}
		errMsg = strings.Join(errorMessages, "\n")
	}
	return errMsg
}

// revokePermissionsToGitRepositoryAzure deletes the managed identity from users list in the organization.
func revokePermissionsToGitRepositoryAzure(ctx context.Context, cfg *gitTestConfig, outputs map[string]*tfjson.StateOutput) error {
	uuid, err := uuid.Parse(cfg.permissionID)
	if err != nil {
		return err
	}

	// Create a connection to the organization and create a new client
	connection := azuredevops.NewPatConnection(fmt.Sprintf("https://dev.azure.com/%s", cfg.organization), cfg.gitPat)
	client, err := memberentitlementmanagement.NewClient(ctx, connection)
	if err != nil {
		return err
	}

	err = client.DeleteServicePrincipalEntitlement(ctx, memberentitlementmanagement.DeleteServicePrincipalEntitlementArgs{ServicePrincipalId: &uuid})
	if err != nil {
		log.Fatal(err)
	}
	cfg.permissionID = ""

	return nil
}

// getGitTestConfigAzure returns the test config used to setup the git repository
func getGitTestConfigAzure(outputs map[string]*tfjson.StateOutput) (*gitTestConfig, error) {
	config := &gitTestConfig{
		defaultGitTransport:   git.HTTP,
		gitUsername:           git.DefaultPublicKeyAuthUser,
		organization:          os.Getenv(envVarAzureDevOpsOrg),
		gitPat:                os.Getenv(envVarAzureDevOpsPAT),
		applicationRepository: outputs["git_repo_url"].Value.(string),
	}

	opts, err := getAuthOpts(config.applicationRepository, map[string][]byte{
		"password": []byte(config.gitPat),
		"username": []byte(git.DefaultPublicKeyAuthUser),
	})
	if err != nil {
		return nil, err
	}
	config.defaultAuthOpts = opts

	parts := strings.Split(config.applicationRepository, "@")
	// Check if the URL contains the "@" symbol
	if len(parts) > 1 {
		// Reconstruct the URL without the username
		config.applicationRepositoryWithoutUser = "https://" + parts[1]
	}

	return config, nil
}
