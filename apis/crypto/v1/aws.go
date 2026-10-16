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
	ProviderAWS = "aws"
)

// Our own AWS-related annotations.
const (
	// AnnotationAWSSTSEndpoint specifies the AWS STS endpoint
	// for the ServiceAccount in the following format:
	//
	//   `^https://(.+\.)?sts(-fips)?(\.[^.]+)?(\.vpce)?\.amazonaws\.com$`
	AnnotationAWSSTSEndpoint AWSSTSEndpoint = GroupAuthAWS + "/sts-endpoint"
)

// AWS-borrowed annotations.
const (
	// AnnotationEKSRoleARN specifies the AWS IAM Role ARN for the ServiceAccount
	// in the following format:
	//
	//   `^arn:aws[\w-]*:iam::[0-9]{1,30}:role/.{1,200}$`
	AnnotationEKSRoleARN EKSRoleARN = GroupAWSEKS + "/role-arn"
)

// Patterns.
const (
	// ClusterPatternAWS is the regex pattern for validating AWS cluster resource names.
	ClusterPatternAWS AWSCluster = `^arn:aws[\w-]*:eks:([^:]{1,100}):[0-9]{1,30}:cluster/(.{1,200})$`
)

var eksRoleARNRegex = regexp.MustCompile(
	`^arn:aws[\w-]*:iam::[0-9]{1,30}:role/.{1,200}$`)

// EKSRoleARN is the type for the AWS IAM Role ARN annotation.
type EKSRoleARN string

// String implements the fmt.Stringer interface.
func (EKSRoleARN) String() string {
	return string(AnnotationEKSRoleARN)
}

// Extract checks if the EKSRoleARN is in the correct format and returns it.
func (EKSRoleARN) Extract(saAnnotations map[string]string) (string, error) {
	const key = string(AnnotationEKSRoleARN)
	regex := eksRoleARNRegex
	roleARN, ok, err := extractServiceAccountAnnotation(key, saAnnotations, regex)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("annotation %s is required", key)
	}
	return roleARN, nil
}

var awsSTSEndpointRegex = regexp.MustCompile(
	`^https://(.+\.)?sts(-fips)?(\.[^.]+)?(\.vpce)?\.amazonaws\.com$`)

// AWSSTSEndpoint is the type for the AWS STS endpoint annotation.
type AWSSTSEndpoint string

// String implements the fmt.Stringer interface.
func (AWSSTSEndpoint) String() string {
	return string(AnnotationAWSSTSEndpoint)
}

// Extract checks if the AWSSTSEndpoint is in the correct format and returns it.
// A spec value is allowed to support the legacy Bucket.spec.sts.endpoint.
func (AWSSTSEndpoint) Extract(saAnnotations map[string]string, specValue string) (string, bool, error) {
	if specValue == "" {
		const key = string(AnnotationAWSSTSEndpoint)
		regex := awsSTSEndpointRegex
		return extractServiceAccountAnnotation(key, saAnnotations, regex)
	}
	rfc0014Annotation, ok := FindRFC0014Annotation(saAnnotations)
	if ok {
		return "", false, fmt.Errorf("legacy Bucket.spec.sts.endpoint for AWS is not allowed when annotation %s is present", rfc0014Annotation)
	}
	return specValue, true, nil
}

var awsClusterPatternRegex = regexp.MustCompile(ClusterPatternAWS.String())

// AWSCluster is the type for the AWS cluster resource name pattern.
type AWSCluster string

// String implements the fmt.Stringer interface.
func (AWSCluster) String() string {
	return string(ClusterPatternAWS)
}

// Extract checks if the AWSCluster is in the correct format and returns it.
func (AWSCluster) ExtractRegionAndName(cluster string) (string, string, error) {
	_, matches, err := parseKubeConfigCluster(cluster, awsClusterPatternRegex)
	if err != nil {
		return "", "", err
	}
	region := matches[1]
	name := matches[2]
	return region, name, nil
}

// GetRoleSessionName returns the AWS IAM Role session name for
// the given ServiceAccount and region. It has the following format:
//
//	`<serviceaccount>.<namespace>.<region>.fluxcd.io`
func GetRoleSessionName(sa, ns, region string) string {
	return fmt.Sprintf("%s.%s.%s.%s", sa, ns, region, GroupCore)
}
