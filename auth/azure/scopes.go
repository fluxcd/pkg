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

const (
	// https://learn.microsoft.com/en-us/azure/devops/integrate/get-started/authentication/service-principal-managed-identity?view=azure-devops#q-can-i-use-a-service-principal-or-managed-identity-with-azure-cli
	ScopeDevOps = "499b84ac-1321-427f-aa17-267ca6975798/.default"

	// https://github.com/Azure/azure-sdk-for-go/blob/f5dfe3b53fe63aacd3aeba948bbe21d961edf376/sdk/storage/azqueue/internal/shared/shared.go#L18
	ScopeBlobStorage = "https://storage.azure.com/.default"

	// https://github.com/Azure/azure-sdk-for-go/blob/f5dfe3b53fe63aacd3aeba948bbe21d961edf376/sdk/messaging/azeventhubs/internal/sbauth/token_provider.go#L99
	ScopeEventHubs = "https://eventhubs.azure.net//.default"
)
