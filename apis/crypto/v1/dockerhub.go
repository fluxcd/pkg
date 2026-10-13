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
)

// Service provider name.
const (
	ProviderDockerHub = "dockerhub"
)

// Our own Docker Hub-related annotations.
const (
	// AnnotationDockerHubOIDCConnectionID specifies the Docker Hub OIDC connection ID
	// for the ServiceAccount in the following format:
	//
	//   `^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	AnnotationDockerHubOIDCConnectionID DockerHubOIDCConnectionID = GroupAuthDockerHub + "/oidc-connection-id"
)

// DockerHubOIDCConnectionID is the type for the Docker Hub OIDC connection ID annotation.
type DockerHubOIDCConnectionID string

// String implements the fmt.Stringer interface.
func (DockerHubOIDCConnectionID) String() string {
	return string(AnnotationDockerHubOIDCConnectionID)
}

// Extract checks if the DockerHubOIDCConnectionID is in the correct format and returns it.
func (DockerHubOIDCConnectionID) Extract(saAnnotations map[string]string) (string, error) {
	const key = string(AnnotationDockerHubOIDCConnectionID)
	regex := uuidRegex
	connectionID, ok, err := extractServiceAccountAnnotation(key, saAnnotations, regex)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("annotation %s is required", key)
	}
	return connectionID, nil
}
