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

// Transit engine annotation keys.
const (
	// AnnotationTransitEngineLoginPath specifies the HTTP path in the transit
	// engine where the request for login must be sent, e.g. `auth/jwt/login`.
	AnnotationTransitEngineLoginPath TransitEngineLoginPath = GroupAuthTransitEngine + "/login-path"

	// AnnotationTransitEngineRole specifies the role to be used when
	// authenticating against the transit engine.
	AnnotationTransitEngineRole TransitEngineRole = GroupAuthTransitEngine + "/role"
)

// ExtractTransitEngineLoginPathAndRole extracts the transit engine login path
// and role from the given ServiceAccount annotations.
//
// These annotations are optional, but they must be specified together: if only
// one of them is set, an error is returned. If neither is set, ok is false, and
// the caller is expected to fall back to the legacy transit engine configuration.
//
// sopsVaultConfigMap indicates whether the kustomize-controller flag
// --sops-vault-configmap is set. This legacy configuration cannot be combined
// with the new RFC-0014 annotations.
func ExtractTransitEngineLoginPathAndRole(saAnnotations map[string]string, sopsVaultConfigMap bool) (string, string, bool, error) {
	if sopsVaultConfigMap {
		if rfc0014Annotation, ok := FindRFC0014Annotation(saAnnotations); ok {
			return "", "", false, fmt.Errorf("legacy --sops-vault-configmap flag is not allowed when annotation %s is present", rfc0014Annotation)
		}
		return "", "", false, nil
	}
	loginPath, loginPathOk, err := AnnotationTransitEngineLoginPath.Extract(saAnnotations)
	if err != nil {
		return "", "", false, err
	}
	role, roleOk, err := AnnotationTransitEngineRole.Extract(saAnnotations)
	if err != nil {
		return "", "", false, err
	}
	if loginPathOk != roleOk {
		return "", "", false, fmt.Errorf("annotations %s and %s must be set together",
			AnnotationTransitEngineLoginPath, AnnotationTransitEngineRole)
	}
	return loginPath, role, loginPathOk, nil
}

var transitEngineLoginPathRegex = regexp.MustCompile(
	`^[a-zA-Z0-9._-]+(/[a-zA-Z0-9._-]+)*$`)

// TransitEngineLoginPath is the type for the transit engine login path annotation.
type TransitEngineLoginPath string

// String implements the fmt.Stringer interface.
func (TransitEngineLoginPath) String() string {
	return string(AnnotationTransitEngineLoginPath)
}

// Extract checks if the TransitEngineLoginPath is valid and returns it.
func (TransitEngineLoginPath) Extract(saAnnotations map[string]string) (string, bool, error) {
	const key = string(AnnotationTransitEngineLoginPath)
	return extractServiceAccountAnnotation(key, saAnnotations, transitEngineLoginPathRegex)
}

var transitEngineRoleRegex = regexp.MustCompile(
	`^[a-zA-Z0-9._-]+$`)

// TransitEngineRole is the type for the transit engine role annotation.
type TransitEngineRole string

// String implements the fmt.Stringer interface.
func (TransitEngineRole) String() string {
	return string(AnnotationTransitEngineRole)
}

// Extract checks if the TransitEngineRole is valid and returns it.
func (TransitEngineRole) Extract(saAnnotations map[string]string) (string, bool, error) {
	const key = string(AnnotationTransitEngineRole)
	return extractServiceAccountAnnotation(key, saAnnotations, transitEngineRoleRegex)
}
