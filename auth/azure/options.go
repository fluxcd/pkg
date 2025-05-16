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
	"regexp"

	corev1 "k8s.io/api/core/v1"
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
