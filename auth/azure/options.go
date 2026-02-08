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

package azure

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
)

const clusterPattern = `(?i)^/subscriptions/([^/]{36})/resourceGroups/([^/]{1,200})/providers/Microsoft\.ContainerService/managedClusters/([^/]{1,200})$`

var clusterRegex = regexp.MustCompile(clusterPattern)

func parseCluster(cluster string) (string, string, string, error) {
	m := clusterRegex.FindStringSubmatch(cluster)
	if len(m) != 4 {
		return "", "", "", fmt.Errorf("invalid AKS cluster ID: '%s'. must match %s",
			cluster, clusterPattern)
	}
	subscriptionID := m[1]
	resourceGroup := m[2]
	clusterName := m[3]
	return subscriptionID, resourceGroup, clusterName, nil
}

// envVarAzureEnvironmentFilepath is the environment variable name used to specify the path of the configuration file with custom Azure endpoints.
const envVarAzureEnvironmentFilepath = "AZURE_ENVIRONMENT_FILEPATH"

// Environment is used to read the Azure environment configuration from a JSON file, it is a subset of the struct defined in
// https://github.com/kubernetes-sigs/cloud-provider-azure/blob/e68bd888a7616d52f45f39238691f32821884120/pkg/azclient/cloud.go#L152-L185
// with exact same field names and json annotations.
// We define this struct here for two reasons:
//  1. We are not aware of any libraries we could import this struct from.
//  2. We don't use all the fields defined in the original struct.
type Environment struct {
	ContainerRegistryDNSSuffix string `json:"containerRegistryDNSSuffix,omitempty"`
	ResourceManagerEndpoint    string `json:"resourceManagerEndpoint,omitempty"`
	TokenAudience              string `json:"tokenAudience,omitempty"`
}

// hasEnvironmentFile checks if the environment variable AZURE_ENVIRONMENT_FILEPATH is set
func hasEnvironmentFile() bool {
	_, ok := os.LookupEnv(envVarAzureEnvironmentFilepath)
	return ok
}

// getEnvironmentConfig reads the Azure environment configuration from a JSON file
// located at the path specified by the environment variable AZURE_ENVIRONMENT_FILEPATH.
// Call hasEnvironmentFile() before calling this function to ensure the file exists.
func getEnvironmentConfig() (*Environment, error) {
	envFilePath := os.Getenv(envVarAzureEnvironmentFilepath)
	if len(envFilePath) == 0 {
		return nil, fmt.Errorf("environment variable %s is not set", envVarAzureEnvironmentFilepath)
	}
	content, err := os.ReadFile(envFilePath)
	if err != nil {
		return nil, err
	}
	env := &Environment{}
	if err = json.Unmarshal(content, env); err != nil {
		return nil, err
	}

	return env, nil
}

// getCloudConfigFromEnvironment reads the Azure environment configuration and returns a cloud.Configuration object.
func getCloudConfigFromEnvironment() (*cloud.Configuration, error) {
	env, err := getEnvironmentConfig()
	if err != nil {
		return nil, err
	}

	cloudConf := cloud.Configuration{
		Services: make(map[cloud.ServiceName]cloud.ServiceConfiguration),
	}
	if len(env.ResourceManagerEndpoint) > 0 && len(env.TokenAudience) > 0 {
		cloudConf.Services[cloud.ResourceManager] = cloud.ServiceConfiguration{
			Endpoint: env.ResourceManagerEndpoint,
			Audience: env.TokenAudience,
		}
	} else {
		return nil, fmt.Errorf("resourceManagerEndpoint and tokenAudience must be set in the environment file")
	}

	return &cloudConf, nil
}

// getContainerRegistryDNSSuffix reads the Azure environment configuration and returns the container registry DNS suffix.
func getContainerRegistryDNSSuffix() (string, error) {
	env, err := getEnvironmentConfig()
	if err != nil {
		return "", err
	}

	if len(env.ContainerRegistryDNSSuffix) == 0 {
		return "", fmt.Errorf("containerRegistryDNSSuffix must be set in the environment file")
	}

	return env.ContainerRegistryDNSSuffix, nil
}
