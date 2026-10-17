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
	"net/url"
	"regexp"
	"strings"
)

// GetAWSRoleSessionName returns the AWS IAM Role session name for the given
// ServiceAccount and region. It has the following format:
//
//	`<serviceaccount>.<namespace>.<region>.fluxcd.io`
func GetAWSRoleSessionName(sa, ns, region string) string {
	return fmt.Sprintf("%s.%s.%s.%s", sa, ns, region, GroupCore)
}

// GetDefaultAudienceGCP returns the default audience for the OIDC token that is
// exchanged for GCP credentials when workload identity federation is configured
// with the given workload identity provider:
//
//	`https://iam.googleapis.com/<workloadIdentityProvider>`
//
// It is the audience used when the credential audiences option is not set.
// Reference:
//
//	https://docs.cloud.google.com/iam/docs/workload-identity-federation-with-other-providers#oidc_1
func GetDefaultAudienceGCP(workloadIdentityProvider string) string {
	return fmt.Sprintf("https://iam.googleapis.com/%s", workloadIdentityProvider)
}

// validateSTSEndpoint checks that the given AWS STS endpoint is valid.
//
// Global and regional endpoints:
//
//	https://docs.aws.amazon.com/general/latest/gr/sts.html
//
// VPC endpoint examples:
//
//	https://vpce-002b7cc8966426bc6-njisq19r.sts.us-east-1.vpce.amazonaws.com
//	https://vpce-002b7cc8966426bc6-njisq19r-us-east-1a.sts.us-east-1.vpce.amazonaws.com
func validateSTSEndpoint(endpoint string) error {
	u, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("invalid STS endpoint: '%s': %w", endpoint, err)
	}
	if u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Opaque != "" ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" ||
		!awsSTSEndpointRegexp.MatchString(u.Host) {
		return fmt.Errorf("invalid STS endpoint: '%s'. must be an https URL with no path, query or credentials, whose host matches %s",
			endpoint, awsSTSEndpointRegexp)
	}
	return nil
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

// splitList parses the given value into a slice of non-empty strings,
// splitting by line breaks and commas, and trimming leading and trailing
// whitespaces.
func splitList(value string) []string {
	var values []string
	for line := range strings.SplitSeq(value, "\n") {
		for part := range strings.SplitSeq(line, ",") {
			if v := strings.TrimSpace(part); v != "" {
				values = append(values, v)
			}
		}
	}
	return values
}

// findNewAnnotation returns the first annotation introduced by this RFC, i.e.
// under the `*.auth.fluxcd.io` or `*.mtls.fluxcd.io` API groups, excluding the
// GCP annotation borrowed from RFC-0010.
func findNewAnnotation(ann map[string]string) (string, bool) {
	return findAnnotation(ann, func(group, key string) bool {
		if key == AnnotationGCPWorkloadIdentityProvider {
			return false
		}
		return strings.HasSuffix(group, "."+GroupAuth) || strings.HasSuffix(group, "."+GroupMTLS)
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
