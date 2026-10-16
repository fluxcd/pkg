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
	"regexp"
)

// Service provider name.
const (
	ProviderAzure = "azure"
)

// Azure-borrowed annotations.
const (
	// AnnotationAzureTenantID specifies the Microsoft Entra tenant ID for the
	// ServiceAccount in the following format:
	//
	//   `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	AnnotationAzureTenantID AzureTenantID = GroupAzureWorkloadIdentity + "/tenant-id"

	// AnnotationAzureClientID specifies the Microsoft Entra client ID for the
	// ServiceAccount in the following format:
	//
	//   `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	AnnotationAzureClientID AzureClientID = GroupAzureWorkloadIdentity + "/client-id"
)

// Patterns.
const (
	// ClusterPatternAzure is the regex pattern for validating Azure cluster resource names.
	ClusterPatternAzure AzureCluster = `(?i)^/subscriptions/([^/]{36})/resourceGroups/([^/]{1,200})/providers/Microsoft\.ContainerService/managedClusters/([^/]{1,200})$`
)

// ExtractAzureIdentity extracts the Azure identity for the given ServiceAccount
// annotations in the following format:
//
//	`<tenant-id>/<client-id>`
func ExtractAzureIdentity(saAnnotations map[string]string) (string, error) {
	tenantID, err := AnnotationAzureTenantID.Extract(saAnnotations)
	if err != nil {
		return "", err
	}
	clientID, err := AnnotationAzureClientID.Extract(saAnnotations)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s", tenantID, clientID), nil
}

// AzureTenantID is the type for the Microsoft Entra tenant ID annotation.
type AzureTenantID string

// String implements the fmt.Stringer interface.
func (AzureTenantID) String() string {
	return string(AnnotationAzureTenantID)
}

// Extract checks if the AzureTenantID is in the correct format and returns it.
func (AzureTenantID) Extract(saAnnotations map[string]string) (string, error) {
	const key = string(AnnotationAzureTenantID)
	regex := uuidRegex
	tenantID, ok, err := extractServiceAccountAnnotation(key, saAnnotations, regex)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("annotation %s is required", key)
	}
	return tenantID, nil
}

// AzureClientID is the type for the Microsoft Entra client ID annotation.
type AzureClientID string

// String implements the fmt.Stringer interface.
func (AzureClientID) String() string {
	return string(AnnotationAzureClientID)
}

// Extract checks if the AzureClientID is in the correct format and returns it.
func (AzureClientID) Extract(saAnnotations map[string]string) (string, error) {
	const key = string(AnnotationAzureClientID)
	regex := uuidRegex
	clientID, ok, err := extractServiceAccountAnnotation(key, saAnnotations, regex)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("annotation %s is required", key)
	}
	return clientID, nil
}

var azureClusterPatternRegex = regexp.MustCompile(ClusterPatternAzure.String())

// AzureCluster is the type for the Azure cluster resource name pattern.
type AzureCluster string

// String implements the fmt.Stringer interface.
func (AzureCluster) String() string {
	return string(ClusterPatternAzure)
}

// ExtractSubscriptionResourceGroupAndName checks if the AzureCluster is in the
// correct format and returns the subscription ID, resource group and cluster name.
func (AzureCluster) ExtractSubscriptionResourceGroupAndName(cluster string) (string, string, string, error) {
	_, matches, err := parseKubeConfigCluster(cluster, azureClusterPatternRegex)
	if err != nil {
		return "", "", "", err
	}
	subscriptionID := matches[1]
	resourceGroup := matches[2]
	name := matches[3]
	return subscriptionID, resourceGroup, name, nil
}
