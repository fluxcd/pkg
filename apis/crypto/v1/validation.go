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
)

// FindRFC0014Annotation checks if a ServiceAccount has any RFC-0014-introduced
// annotations and returns the first one it finds.
func FindRFC0014Annotation(saAnnotations map[string]string) (string, bool) {
	for key := range saAnnotations {
		group, _, ok := strings.Cut(key, "/")
		if !ok || !strings.HasSuffix(group, "."+GroupCore) {
			continue
		}
		if key != AnnotationGCPWorkloadIdentityProvider.String() {
			return key, true
		}
	}
	return "", false
}

// uuidRegex matches a canonical (hyphenated) UUID. It deliberately does not
// enforce a specific version or variant: cloud providers issue UUID-formatted
// identifiers that are not guaranteed to be version 4 (e.g. some Microsoft
// Entra client IDs), and stricter callers can layer their own checks on top.
var uuidRegex = regexp.MustCompile(
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// extractServiceAccountAnnotation validates annotations used in a ServiceAccount.
func extractServiceAccountAnnotation(key string, saAnnotations map[string]string, regex *regexp.Regexp) (string, bool, error) {
	value, ok := saAnnotations[key]
	if !ok {
		return "", false, nil
	}
	s, _, err := parseValueFromMap("ServiceAccount annotation key", key, value, regex)
	if err != nil {
		return "", false, err
	}
	return s, true, nil
}

// parseKubeConfigCluster validates keys used in a ConfigMap.
func parseKubeConfigCluster(value string, regex *regexp.Regexp) (string, []string, error) {
	return parseValueFromMap("kubeconfig ConfigMap data key", "cluster", value, regex)
}

func parseValueFromMap(keyType, key, value string, regex *regexp.Regexp) (string, []string, error) {
	matches := regex.FindStringSubmatch(value)
	if matches == nil {
		return "", nil, fmt.Errorf("invalid %s %s: '%s'. must match %s",
			keyType, key, value, regex.String())
	}
	return value, matches, nil
}
