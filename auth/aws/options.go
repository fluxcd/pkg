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
	"errors"
	"fmt"
	"regexp"

	corev1 "k8s.io/api/core/v1"

	"github.com/fluxcd/pkg/auth"
)

const stsEndpointPattern = `^https://(.+\.)?sts(-fips)?(\.[^.]+)?(\.vpce)?\.amazonaws\.com$`

var stsEndpointRegex = regexp.MustCompile(stsEndpointPattern)

// ValidateSTSEndpoint checks if the provided STS endpoint is valid.
//
// Global and regional endpoints:
//
//	https://docs.aws.amazon.com/general/latest/gr/sts.html
//
// VPC endpoint examples:
//
//	https://vpce-002b7cc8966426bc6-njisq19r.sts.us-east-1.vpce.amazonaws.com
//	https://vpce-002b7cc8966426bc6-njisq19r-us-east-1a.sts.us-east-1.vpce.amazonaws.com
func ValidateSTSEndpoint(endpoint string) error {
	if !stsEndpointRegex.MatchString(endpoint) {
		return fmt.Errorf("invalid STS endpoint: '%s'. must match %s",
			endpoint, stsEndpointPattern)
	}
	return nil
}

const roleARNPattern = `^arn:aws[\w-]*:iam::[0-9]{1,30}:role/(.{1,200})$`

var roleARNRegex = regexp.MustCompile(roleARNPattern)

func getRoleARN(serviceAccount corev1.ServiceAccount) (string, error) {
	const key = "eks.amazonaws.com/role-arn"
	arn := serviceAccount.Annotations[key]
	if !roleARNRegex.MatchString(arn) {
		return "", fmt.Errorf("invalid %s annotation: '%s'. must match %s",
			key, arn, roleARNPattern)
	}
	return arn, nil
}

func getRoleNameFromARN(arn string) (string, error) {
	m := roleARNRegex.FindStringSubmatch(arn)
	if len(m) != 2 {
		return "", fmt.Errorf("invalid role ARN: '%s'. must match %s",
			arn, roleARNPattern)
	}
	return m[1], nil
}

func getRoleSessionNameForServiceAccount(serviceAccount corev1.ServiceAccount, region string) string {
	name := serviceAccount.Name
	namespace := serviceAccount.Namespace
	return fmt.Sprintf("%s.%s.%s.fluxcd.io", name, namespace, region)
}

func getRoleSessionNameForImpersonation(roleName, region string) string {
	return fmt.Sprintf("%s.%s.fluxcd.io", roleName, region)
}

const clusterPattern = `^arn:aws[\w-]*:eks:([^:]{1,100}):[0-9]{1,30}:cluster/(.{1,200})$`

var clusterRegex = regexp.MustCompile(clusterPattern)

func parseCluster(cluster string) (string, string, error) {
	m := clusterRegex.FindStringSubmatch(cluster)
	if len(m) != 3 {
		return "", "", fmt.Errorf("invalid EKS cluster ARN: '%s'. must match %s",
			cluster, clusterPattern)
	}
	region := m[1]
	name := m[2]
	return region, name, nil
}

func getSTSRegionForObjectLevel(o *auth.Options) (string, error) {
	stsRegion := o.STSRegion
	if stsRegion == "" {
		// In this case we can't rely on IRSA or EKS Pod Identity for the controller
		// pod because this is object-level configuration, so we show a different
		// error message.
		// In this error message we assume an API that has a region field, e.g. the
		// Bucket API. APIs that can extract the region from the ARN (e.g. KMS) will
		// never reach this code path.
		return "", errors.New("an AWS region is required for authenticating with a service account. " +
			"please configure one in the object spec")
	}
	return stsRegion, nil
}
