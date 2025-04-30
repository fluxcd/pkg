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

package aws

import (
	"fmt"
	"os"
	"regexp"

	corev1 "k8s.io/api/core/v1"
)

func getRegion() string {
	// The AWS_REGION is usually automatically set in EKS clusters.
	// If not set users can set it manually (e.g. Fargate).
	return os.Getenv("AWS_REGION")
}

const roleARNPattern = `^arn:aws:iam::[0-9]{1,30}:role/.{1,200}$`

var roleARNRegex = regexp.MustCompile(roleARNPattern)

func getRoleARN(serviceAccount corev1.ServiceAccount) (string, error) {
	arn := serviceAccount.Annotations["eks.amazonaws.com/role-arn"]
	if !roleARNRegex.MatchString(arn) {
		return "", fmt.Errorf("invalid AWS role ARN: '%s'. must match %s",
			arn, roleARNPattern)
	}
	return arn, nil
}

func getRoleSessionName(serviceAccount corev1.ServiceAccount) string {
	name := serviceAccount.Name
	namespace := serviceAccount.Namespace
	region := getRegion()
	return fmt.Sprintf("%s.%s.%s.fluxcd.io", name, namespace, region)
}

// This regex is sourced from the AWS ECR Credential Helper (https://github.com/awslabs/amazon-ecr-credential-helper).
// It covers both public AWS partitions like amazonaws.com, China partitions like amazonaws.com.cn, and non-public partitions.
var registryPartRe = regexp.MustCompile(`([0-9+]*).dkr.ecr(?:-fips)?\.([^/.]*)\.(amazonaws\.com[.cn]*|sc2s\.sgov\.gov|c2s\.ic\.gov|cloud\.adc-e\.uk|csp\.hci\.ic\.gov)`)

// ParseRegistry returns the AWS account ID and region and `true` if
// the image registry/repository is hosted in AWS's Elastic Container Registry,
// otherwise empty strings and `false`.
func ParseRegistry(registry string) (accountId, awsEcrRegion string, ok bool) {
	registryParts := registryPartRe.FindAllStringSubmatch(registry, -1)
	if len(registryParts) < 1 || len(registryParts[0]) < 3 {
		return "", "", false
	}
	return registryParts[0][1], registryParts[0][2], true
}
