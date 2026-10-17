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

// GetRoleSessionName returns the AWS IAM Role session name for the given
// ServiceAccount and region. It has the following format:
//
//	`<serviceaccount>.<namespace>.<region>.fluxcd.io`
func GetRoleSessionName(sa, ns, region string) string {
	return fmt.Sprintf("%s.%s.%s.%s", sa, ns, region, GroupCore)
}

// extract returns the value of the given annotation validated against regex, or
// false when the annotation is not set.
func extract(ann map[string]string, key string, regex *regexp.Regexp) (string, bool, error) {
	value, ok := ann[key]
	if !ok {
		return "", false, nil
	}
	if !regex.MatchString(value) {
		return "", false, fmt.Errorf("invalid ServiceAccount annotation key %s: '%s'. must match %s",
			key, value, regex)
	}
	return value, true, nil
}

// parseCluster validates the given kubeconfig cluster resource name against
// regex and returns the captured groups.
func parseCluster(cluster string, regex *regexp.Regexp) ([]string, error) {
	matches := regex.FindStringSubmatch(cluster)
	if matches == nil {
		return nil, fmt.Errorf("invalid kubeconfig ConfigMap data key cluster: '%s'. must match %s",
			cluster, regex)
	}
	return matches, nil
}

// splitAudiences parses the given value into a slice of non-empty strings,
// splitting by line breaks and commas, and trimming leading and trailing
// whitespaces.
func splitAudiences(value string) []string {
	var audiences []string
	for line := range strings.SplitSeq(value, "\n") {
		for part := range strings.SplitSeq(line, ",") {
			if aud := strings.TrimSpace(part); aud != "" {
				audiences = append(audiences, aud)
			}
		}
	}
	return audiences
}

// findNewAnnotation returns the first annotation introduced by this RFC, i.e.
// under the `*.fluxcd.io` domain, excluding the GCP annotation borrowed from
// RFC-0010.
func findNewAnnotation(ann map[string]string) (string, bool) {
	return findAnnotation(ann, func(group, key string) bool {
		return key != AnnotationGCPWorkloadIdentityProvider && strings.HasSuffix(group, "."+GroupCore)
	})
}

// findNewAuthAnnotation returns the first authentication annotation introduced
// by this RFC, i.e. under the `*.auth.fluxcd.io` API group, excluding the GCP
// annotation borrowed from RFC-0010.
func findNewAuthAnnotation(ann map[string]string) (string, bool) {
	return findAnnotation(ann, func(group, key string) bool {
		return key != AnnotationGCPWorkloadIdentityProvider && strings.HasSuffix(group, "."+GroupAuth)
	})
}

// findAuthAnnotation returns the first annotation that configures
// credential acquisition, whether introduced by this RFC or borrowed from
// RFC-0010 and the cloud providers.
func findAuthAnnotation(ann map[string]string) (string, bool) {
	return findAnnotation(ann, func(group, _ string) bool {
		if strings.HasSuffix(group, "."+GroupAuth) {
			return true
		}
		switch group {
		case GroupAWSEKS, GroupAzureWorkloadIdentity, GroupGKEIAM:
			return true
		}
		return false
	})
}

// findAnnotation returns the first annotation key for which match returns true.
func findAnnotation(ann map[string]string, match func(group, key string) bool) (string, bool) {
	for key := range ann {
		group, _, ok := strings.Cut(key, "/")
		if !ok {
			continue
		}
		if match(group, key) {
			return key, true
		}
	}
	return "", false
}
