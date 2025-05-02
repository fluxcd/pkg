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
	"os"
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
	if tenantID, ok := serviceAccount.Annotations["azure.workload.identity/tenant-id"]; ok {
		return tenantID, nil
	}
	if tenantID := os.Getenv("AZURE_TENANT_ID"); tenantID != "" {
		return tenantID, nil
	}
	return "", fmt.Errorf("azure tenant ID not found in the service account annotations nor in the environment variable AZURE_TENANT_ID")
}

func getClientID(serviceAccount corev1.ServiceAccount) (string, error) {
	if clientID, ok := serviceAccount.Annotations["azure.workload.identity/client-id"]; ok {
		return clientID, nil
	}
	return "", fmt.Errorf("azure client ID not found in the service account annotations")
}

func getScopes(o *auth.Options) []string {
	if ar := o.ArtifactRepository; ar != "" {
		return []string{getACRScope(ar)}
	}
	return o.Scopes
}

func getACRScope(artifactRepository string) string {
	var conf *cloud.Configuration
	switch {
	case strings.HasSuffix(artifactRepository, ".azurecr.cn"):
		conf = &cloud.AzureChina
	case strings.HasSuffix(artifactRepository, ".azurecr.us"):
		conf = &cloud.AzureGovernment
	default:
		conf = &cloud.AzurePublic
	}
	return conf.Services[cloud.ResourceManager].Endpoint + "/" + ".default"
}

func getACRHost(artifactRepository string) string {
	return strings.SplitN(artifactRepository, "/", 2)[0]
}
