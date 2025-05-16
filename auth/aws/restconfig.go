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
	"context"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	signerv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// eksHTTPPresignerV4 implements sts.HTTPPresignerV4 adding the cluster name
// to the request header x-k8s-aws-id, as required by EKS authentication.
type eksHTTPPresignerV4 struct {
	sts.HTTPPresignerV4
	clusterName string
}

// PresignHTTP implements sts.HTTPPresignerV4.
func (e *eksHTTPPresignerV4) PresignHTTP(
	ctx context.Context, credentials aws.Credentials, r *http.Request,
	payloadHash string, service string, region string, signingTime time.Time,
	optFns ...func(*signerv4.SignerOptions),
) (string, http.Header, error) {
	r.Header.Add("x-k8s-aws-id", e.clusterName)
	r.Header.Add("X-Amz-Expires", "900") // ref: https://github.com/aws/aws-sdk-go-v2/issues/1922#issuecomment-1429063756
	return e.HTTPPresignerV4.PresignHTTP(ctx, credentials, r, payloadHash, service, region, signingTime, optFns...)
}
