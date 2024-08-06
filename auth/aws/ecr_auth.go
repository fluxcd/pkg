/*
Copyright 2023 The Flux authors

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
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/google/go-containerregistry/pkg/authn"
)

var registryPartRe = regexp.MustCompile(`([0-9+]*).dkr.ecr.([^/.]*)\.(amazonaws\.com[.cn]*)`)

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

// GetECRAuthConfig returns an AuthConfig that contains the credentials
// required to authenticate against ECR to access the provided image.
func (p *Provider) GetECRAuthConfig(ctx context.Context, image string) (authn.AuthConfig, time.Duration, error) {
	var authConfig authn.AuthConfig
	var expiresIn time.Duration

	_, awsEcrRegion, ok := ParseRegistry(image)
	if !ok {
		return authConfig, expiresIn, errors.New("failed to parse AWS ECR image, invalid ECR image")
	}
	p.optFns = append(p.optFns, config.WithRegion(awsEcrRegion))

	cfg, err := p.GetConfig(ctx)
	if err != nil {
		return authConfig, expiresIn, err
	}

	ecrService := ecr.NewFromConfig(cfg)
	// NOTE: ecr.GetAuthorizationTokenInput has deprecated RegistryIds. Hence,
	// pass nil input.
	ecrToken, err := ecrService.GetAuthorizationToken(ctx, nil)
	if err != nil {
		return authConfig, expiresIn, err
	}

	// Validate the authorization data.
	if len(ecrToken.AuthorizationData) == 0 {
		return authConfig, expiresIn, errors.New("no authorization data")
	}
	authData := ecrToken.AuthorizationData[0]
	if authData.AuthorizationToken == nil {
		return authConfig, expiresIn, fmt.Errorf("no authorization token")
	}
	token, err := base64.StdEncoding.DecodeString(*authData.AuthorizationToken)
	if err != nil {
		return authConfig, expiresIn, err
	}

	tokenSplit := strings.Split(string(token), ":")
	// Validate the tokens.
	if len(tokenSplit) != 2 {
		return authConfig, expiresIn, fmt.Errorf("invalid authorization token, expected the token to have two parts separated by ':', got %d parts", len(tokenSplit))
	}

	authConfig = authn.AuthConfig{
		Username: tokenSplit[0],
		Password: tokenSplit[1],
	}
	if authData.ExpiresAt == nil {
		return authConfig, expiresIn, fmt.Errorf("no expiration time")
	}
	expiresIn = authData.ExpiresAt.Sub(time.Now())

	return authConfig, expiresIn, nil
}
