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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// newDefaultAzureCredential is like azidentity.NewDefaultAzureCredential(),
// but does not call the functions that shell out to Azure CLIs.
func newDefaultAzureCredential(options *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
	const (
		azureClientID           = "AZURE_CLIENT_ID"
		azureFederatedTokenFile = "AZURE_FEDERATED_TOKEN_FILE"
		azureAuthorityHost      = "AZURE_AUTHORITY_HOST"
		azureTenantID           = "AZURE_TENANT_ID"
	)

	var errorMessages []string

	envCred, err := azidentity.NewEnvironmentCredential(&azidentity.EnvironmentCredentialOptions{
		ClientOptions: options.ClientOptions, DisableInstanceDiscovery: options.DisableInstanceDiscovery},
	)
	if err == nil {
		return envCred, nil
	} else {
		errorMessages = append(errorMessages, "EnvironmentCredential: "+err.Error())
	}

	// workload identity requires values for AZURE_AUTHORITY_HOST, AZURE_CLIENT_ID, AZURE_FEDERATED_TOKEN_FILE, AZURE_TENANT_ID
	haveWorkloadConfig := false
	clientID, haveClientID := os.LookupEnv(azureClientID)
	if haveClientID {
		if file, ok := os.LookupEnv(azureFederatedTokenFile); ok {
			if _, ok := os.LookupEnv(azureAuthorityHost); ok {
				if tenantID, ok := os.LookupEnv(azureTenantID); ok {
					haveWorkloadConfig = true
					workloadCred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
						ClientID:                 clientID,
						TenantID:                 tenantID,
						TokenFilePath:            file,
						ClientOptions:            options.ClientOptions,
						DisableInstanceDiscovery: options.DisableInstanceDiscovery,
					})
					if err == nil {
						return workloadCred, nil
					} else {
						errorMessages = append(errorMessages, "Workload Identity"+": "+err.Error())
					}
				}
			}
		}
	}
	if !haveWorkloadConfig {
		err := errors.New("missing environment variables for workload identity. Check webhook and pod configuration")
		errorMessages = append(errorMessages, fmt.Sprintf("Workload Identity: %s", err))
	}

	o := &azidentity.ManagedIdentityCredentialOptions{ClientOptions: options.ClientOptions}
	if haveClientID {
		o.ID = azidentity.ClientID(clientID)
	}
	miCred, err := azidentity.NewManagedIdentityCredential(o)
	if err == nil {
		return miCred, nil
	} else {
		errorMessages = append(errorMessages, "ManagedIdentity"+": "+err.Error())
	}

	return nil, errors.New(strings.Join(errorMessages, "\n"))
}
