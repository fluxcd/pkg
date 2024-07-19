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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/fluxcd/pkg/git"
	"github.com/fluxcd/test-infra/tftestenv"
	tfjson "github.com/hashicorp/terraform-json"
)

const (
	// azureWIClientIdAnnotation is the key for the annotation on the kubernetes serviceaccount
	azureWIClientIdAnnotation = "azure.workload.identity/client-id"
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

	return map[string]string{
		azureWIClientIdAnnotation: clientID,
	}, nil
}

// Give managed identity permissions on the azure devops project using
// ServicePrincipalEntitlement REST API
// https://learn.microsoft.com/en-us/rest/api/azure/devops/memberentitlementmanagement/service-principal-entitlements/add?view=azure-devops-rest-7.1&tabs=HTTP
// This can be moved to terraform if/when this PR completes -
// https://github.com/microsoft/terraform-provider-azuredevops/pull/1028
type ServicePrincipalEntitlement struct {
	AccessLevel struct {
		AccountLicenseType string `json:"accountLicenseType"`
	} `json:"accessLevel"`
	ProjectEntitlements []struct {
		Group struct {
			GroupType string `json:"groupType"`
		} `json:"group"`
		ProjectRef struct {
			ID string `json:"id"`
		} `json:"projectRef"`
	} `json:"projectEntitlements"`
	ServicePrincipal struct {
		Origin      string `json:"origin"`
		OriginID    string `json:"originId"`
		SubjectKind string `json:"subjectKind"`
	} `json:"servicePrincipal"`
}

func givePermissionsToRepositoryAzure(outputs map[string]*tfjson.StateOutput) error {
	// Organization, PAT, Project ID and WI ID are availble as terraform output
	organization := outputs["azure_devops_organization"].Value.(string)
	project_id := outputs["azure_devops_project_id"].Value.(string)
	pat := outputs["azure_devops_access_token"].Value.(string)
	wi_object_id := outputs["workload_identity_object_id"].Value.(string)

	encodedPat := base64.StdEncoding.EncodeToString([]byte(":" + pat))
	apiURL := fmt.Sprintf("https://vsaex.dev.azure.com/%s/_apis/serviceprincipalentitlements?api-version=7.1-preview.1", organization)

	// Set up the request payload
	payload := ServicePrincipalEntitlement{
		AccessLevel: struct {
			AccountLicenseType string `json:"accountLicenseType"`
		}{
			AccountLicenseType: "express",
		},
		ProjectEntitlements: []struct {
			Group struct {
				GroupType string `json:"groupType"`
			} `json:"group"`
			ProjectRef struct {
				ID string `json:"id"`
			} `json:"projectRef"`
		}{
			{
				Group: struct {
					GroupType string `json:"groupType"`
				}{
					GroupType: "projectContributor",
				},
				ProjectRef: struct {
					ID string `json:"id"`
				}{
					ID: project_id,
				},
			},
		},
		ServicePrincipal: struct {
			Origin      string `json:"origin"`
			OriginID    string `json:"originId"`
			SubjectKind string `json:"subjectKind"`
		}{
			Origin:      "aad",
			OriginID:    wi_object_id,
			SubjectKind: "servicePrincipal",
		},
	}

	// Marshal the payload into JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling the payload:%v", err)
		return err
	}

	// First request to add user always fails, second request succeeds, add a
	// retry
	retryAttempts := 3
	retryDelay := 5 * time.Second // 5 seconds delay
	attempts := 0

	for attempts < retryAttempts {
		attempts++
		// Create a new request
		req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(jsonPayload))
		if err != nil {
			log.Printf("Error creating the request: %v", err)
			return err
		}

		// Set the authorization header to use the PAT
		req.Header.Set("Authorization", "Basic "+strings.TrimSpace(encodedPat))
		req.Header.Set("Content-Type", "application/json")

		// Send the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Error sending the request: %v", err)
			return err
		}
		defer resp.Body.Close()

		// Read the response body
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil || strings.Contains(string(body), "VS403283: Could not add user") {
			log.Printf("Encountered error : %v, retrying..", err)
			time.Sleep(retryDelay)
			continue
		}

		log.Printf("Added managed identity to organization:")
		break
	}
	return nil
}

// getTestConfigAzure returns the test config used to setup the git repository
func getTestConfigAzure(outputs map[string]*tfjson.StateOutput) (*testConfig, error) {
	config := &testConfig{
		defaultGitTransport:   git.HTTP,
		gitUsername:           git.DefaultPublicKeyAuthUser,
		gitPat:                outputs["azure_devops_access_token"].Value.(string),
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

	fmt.Println("URL without username:", config.applicationRepositoryWithoutUser)
	return config, nil
}
