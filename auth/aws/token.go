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
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"
)

// Credentials is the AWS token.
type Credentials struct{ types.Credentials }

func newTokenFromAWSCredentials(creds *aws.Credentials) *Credentials {
	return &Credentials{types.Credentials{
		AccessKeyId:     &creds.AccessKeyID,
		SecretAccessKey: &creds.SecretAccessKey,
		SessionToken:    &creds.SessionToken,
		Expiration:      &creds.Expires,
	}}
}

// GetDuration implements auth.Token.
func (c *Credentials) GetDuration() time.Duration {
	return time.Until(*c.Expiration)
}

func (c *Credentials) provider() aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider(*c.AccessKeyId, *c.SecretAccessKey, *c.SessionToken)
}
