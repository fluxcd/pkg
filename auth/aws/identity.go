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

package aws

import (
	"encoding/json"
)

// Identity represents an AWS identity that can be impersonated.
type Identity struct {
	RoleARN string `json:"roleARN"`
}

// String implements auth.Identity.
func (i *Identity) String() string {
	return i.RoleARN
}

// UnmarshalJSON implements json.Unmarshaler to validate the identity format when unmarshaling.
func (i *Identity) UnmarshalJSON(data []byte) error {
	type alias Identity
	var id alias
	if err := json.Unmarshal(data, &id); err != nil {
		return err
	}
	if err := parseRoleARN(id.RoleARN); err != nil {
		return err
	}
	*i = Identity(id)
	return nil
}
