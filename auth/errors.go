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

package auth

import (
	"errors"
	"fmt"
)

// ErrNoAudienceForOIDCImpersonation is returned when no audience for OIDC impersonation
// is found in the options or service account annotations.
var ErrNoAudienceForOIDCImpersonation = errors.New(
	"no audience for OIDC impersonation found in options or service account annotations")

// ErrNoIdentityForOIDCImpersonation is returned when no identity for OIDC impersonation
// is found in the options or service account annotations.
var ErrNoIdentityForOIDCImpersonation = errors.New(
	"no identity for OIDC impersonation found in options or service account annotations")

// ErrInvalidIdentityType is returned when a provider observes an identity type which is
// not its own identity type.
func ErrInvalidIdentityType(want, got Identity) error {
	return fmt.Errorf("invalid identity type: want %T, got %T", want, got)
}
