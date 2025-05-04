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
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/auth"
)

func getIdentity(serviceAccount corev1.ServiceAccount) (string, error) {
	tenantID, err := getTenantID(serviceAccount)
	if err != nil {
		return "", err
	}
	clientID, err := getClientID(serviceAccount)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", tenantID, clientID), nil
}

func getTenantID(serviceAccount corev1.ServiceAccount) (string, error) {
	const key = "azure.workload.identity/tenant-id"
	if tenantID, ok := serviceAccount.Annotations[key]; ok {
		return tenantID, nil
	}
	return "", fmt.Errorf("azure tenant ID is not set in the service account annotation %s", key)
}

func getClientID(serviceAccount corev1.ServiceAccount) (string, error) {
	const key = "azure.workload.identity/client-id"
	if clientID, ok := serviceAccount.Annotations[key]; ok {
		return clientID, nil
	}
	return "", fmt.Errorf("azure client ID is not set in the service account annotation %s", key)
}

func getScopes(o *auth.Options) []string {
	if acrScope := getACRScope(o.ArtifactRepository); acrScope != "" {
		return []string{acrScope}
	}
	return o.Scopes
}

func getACRScope(artifactRepository string) string {
	if artifactRepository == "" {
		return ""
	}

	registry, err := auth.GetRegistryFromArtifactRepository(artifactRepository)
	if err != nil {
		// it's ok to swallow the error here, it should never happen
		// because GetRegistryFromArtifactRepository() is already called
		// earlier by auth.GetToken() and the error is handled there.
		return ""
	}

	var conf *cloud.Configuration
	switch {
	case strings.HasSuffix(registry, ".azurecr.cn"):
		conf = &cloud.AzureChina
	case strings.HasSuffix(registry, ".azurecr.us"):
		conf = &cloud.AzureGovernment
	default:
		conf = &cloud.AzurePublic
	}
	return conf.Services[cloud.ResourceManager].Endpoint + "/" + ".default"
}
