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
	"strings"
	"time"
)

// Credential provider values.
const (
	// CredentialProviderKubernetes is the credential provider that uses the
	// Kubernetes TokenRequest API for acquiring the credential.
	CredentialProviderKubernetes = "kubernetes"

	// CredentialProviderSPIFFE is the credential provider that uses the
	// SPIFFE Broker API for acquiring the credential.
	CredentialProviderSPIFFE = "spiffe"
)

// Credential type values.
const (
	// CredentialTypeJWT is the credential type for JSON Web Tokens.
	CredentialTypeJWT = "jwt"

	// CredentialTypeX509 is the credential type for X.509 certificates.
	CredentialTypeX509 = "x509"
)

// Credential annotation keys.
const (
	// AnnotationCredentialProvider specifies the provider that issues the
	// credential. It must be one of `kubernetes` or `spiffe`.
	AnnotationCredentialProvider CredentialProvider = GroupAuthCredential + "/provider"

	// AnnotationCredentialType specifies the type of the credential.
	// It must be one of `jwt` or `x509`.
	AnnotationCredentialType CredentialType = GroupAuthCredential + "/type"

	// AnnotationCredentialExpiration specifies the requested expiration time
	// of the credential as a Go duration string, e.g. `5m` for five minutes
	// or `1h` for one hour. It is supported only by the `kubernetes` credential
	// provider.
	AnnotationCredentialExpiration CredentialExpiration = GroupAuthCredential + "/expiration"

	// AnnotationCredentialUsername specifies an optional username to use when
	// authenticating with the service.
	AnnotationCredentialUsername CredentialUsername = GroupAuthCredential + "/username"

	// AnnotationCredentialAudiences is the optional annotation key for specifying
	// the target audiences of a credential. The system receiving the credential
	// validates these values. The list of strings extracted from this annotation
	// is the set of non-empty strings observed from splitting a multi-line value
	// by line breaks into lines that are further splitted by commas and then
	// trimmed of leading and trailing whitespaces. Example:
	//   credential.auth.fluxcd.io/audiences: |
	//     audience1,audience2
	//
	//     audience3
	//     audience4,audience5 , audience6
	// Results in:
	//   - `audience1`
	//   - `audience2`
	//   - `audience3`
	//   - `audience4`
	//   - `audience5`
	//   - `audience6`
	AnnotationCredentialAudiences CredentialAudiences = GroupAuthCredential + "/audiences"
)

// ExtractCredentialProviderAndType extracts the credential provider and type
// for the given service provider from the given ServiceAccount annotations.
//
// These annotations are optional, but they must be specified together: if only
// one of them is set, an error is returned. If neither is set, ok is false, and
// the caller is expected to assume the backward-compatible defaults `kubernetes`
// and `jwt`. The `kubernetes` credential provider supports only the `jwt`
// credential type, and the `azure` service provider does not support the `x509`
// credential type.
func ExtractCredentialProviderAndType(saAnnotations map[string]string, specProvider string) (string, string, bool, error) {
	credProvider, providerOk, err := AnnotationCredentialProvider.Extract(saAnnotations)
	if err != nil {
		return "", "", false, err
	}
	credType, typeOk, err := AnnotationCredentialType.Extract(saAnnotations)
	if err != nil {
		return "", "", false, err
	}
	if providerOk != typeOk {
		return "", "", false, fmt.Errorf("annotations %s and %s must be set together",
			AnnotationCredentialProvider, AnnotationCredentialType)
	}
	if credProvider == CredentialProviderKubernetes && credType != CredentialTypeJWT {
		return "", "", false, fmt.Errorf("credential type %s is not supported by the %s credential provider",
			credType, credProvider)
	}
	if specProvider == ProviderAzure && credType == CredentialTypeX509 {
		return "", "", false, fmt.Errorf("credential type %s is not supported by the %s provider",
			credType, specProvider)
	}
	return credProvider, credType, providerOk, nil
}

var credentialProviderRegex = regexp.MustCompile(
	`^(kubernetes|spiffe)$`)

// CredentialProvider is the type for the credential provider annotation.
type CredentialProvider string

// String implements the fmt.Stringer interface.
func (CredentialProvider) String() string {
	return string(AnnotationCredentialProvider)
}

// Extract checks if the CredentialProvider is valid and returns it.
func (CredentialProvider) Extract(saAnnotations map[string]string) (string, bool, error) {
	const key = string(AnnotationCredentialProvider)
	return extractServiceAccountAnnotation(key, saAnnotations, credentialProviderRegex)
}

var credentialTypeRegex = regexp.MustCompile(
	`^(jwt|x509)$`)

// CredentialType is the type for the credential type annotation.
type CredentialType string

// String implements the fmt.Stringer interface.
func (CredentialType) String() string {
	return string(AnnotationCredentialType)
}

// Extract checks if the CredentialType is valid and returns it.
func (CredentialType) Extract(saAnnotations map[string]string) (string, bool, error) {
	const key = string(AnnotationCredentialType)
	return extractServiceAccountAnnotation(key, saAnnotations, credentialTypeRegex)
}

// CredentialExpiration is the type for the credential expiration annotation.
type CredentialExpiration string

// String implements the fmt.Stringer interface.
func (CredentialExpiration) String() string {
	return string(AnnotationCredentialExpiration)
}

// Extract parses the CredentialExpiration as a Go duration string and returns
// the parsed duration.
func (CredentialExpiration) Extract(saAnnotations map[string]string) (time.Duration, bool, error) {
	const key = string(AnnotationCredentialExpiration)
	value, ok := saAnnotations[key]
	if !ok {
		return 0, false, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, false, fmt.Errorf("invalid ServiceAccount annotation key %s: '%s': %w",
			key, value, err)
	}
	if duration <= 0 {
		return 0, false, fmt.Errorf("invalid ServiceAccount annotation key %s: '%s'. duration must be positive",
			key, value)
	}
	return duration, true, nil
}

var credentialUsernameRegex = regexp.MustCompile(
	`^\S+$`)

// CredentialUsername is the type for the credential username annotation.
type CredentialUsername string

// String implements the fmt.Stringer interface.
func (CredentialUsername) String() string {
	return string(AnnotationCredentialUsername)
}

// Extract checks if the CredentialUsername is valid and returns it.
func (CredentialUsername) Extract(saAnnotations map[string]string) (string, bool, error) {
	const key = string(AnnotationCredentialUsername)
	return extractServiceAccountAnnotation(key, saAnnotations, credentialUsernameRegex)
}

// CredentialAudiences is the type for the credential audiences annotation.
type CredentialAudiences string

// String implements the fmt.Stringer interface.
func (CredentialAudiences) String() string {
	return string(AnnotationCredentialAudiences)
}

// Extract checks if the CredentialAudiences are valid and returns them as a slice of strings.
func (CredentialAudiences) Extract(saAnnotations map[string]string) []string {
	saAudiences, ok := saAnnotations[AnnotationCredentialAudiences.String()]
	if !ok {
		return nil
	}
	return AnnotationCredentialAudiences.extract(saAudiences)
}

// ExtractKubeConfigAudiences extracts the audiences for minting a credential for
// accessing a remote Kubernetes cluster in applier APIs from the given ServiceAccount
// annotations and ConfigMap data. It implements the rule defined by KubeConfigLegacyAudiences.
func (CredentialAudiences) ExtractKubeConfigAudiences(saAnnotations, cmData map[string]string) ([]string, error) {
	cmAudiences, ok := cmData[KubeConfigLegacyAudiences]
	if !ok {
		return AnnotationCredentialAudiences.Extract(saAnnotations), nil
	}
	rfc0014Annotation, ok := FindRFC0014Annotation(saAnnotations)
	if ok {
		return nil, fmt.Errorf("legacy audiences Config data key is not allowed when annotation %s is present", rfc0014Annotation)
	}
	return AnnotationCredentialAudiences.extract(cmAudiences), nil
}

// extract parses the given value into a slice of non-empty strings,
// splitting by line breaks and commas, and trimming leading and
// trailing whitespaces.
func (CredentialAudiences) extract(value string) []string {
	var audiences []string
	for line := range strings.SplitSeq(value, "\n") {
		for part := range strings.SplitSeq(line, ",") {
			aud := strings.TrimSpace(part)
			if aud != "" {
				audiences = append(audiences, aud)
			}
		}
	}
	return audiences
}
