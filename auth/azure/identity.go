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

package azure

import (
	"encoding/json"
	"fmt"
)

// Identity represents an Azure identity that can be impersonated.
type Identity struct {
	ClientID string `json:"clientID"`
	TenantID string `json:"tenantID"`
}

// String implements auth.Identity.
func (i *Identity) String() string {
	return fmt.Sprintf("%s/%s", i.TenantID, i.ClientID)
}

// Validate validates the identity fields.
func (i *Identity) Validate() error {
	if i.ClientID == "" {
		return fmt.Errorf("clientID is required")
	}
	if i.TenantID == "" {
		return fmt.Errorf("tenantID is required")
	}
	return nil
}

// UnmarshalJSON implements json.Unmarshaler to validate the identity format when unmarshaling.
func (i *Identity) UnmarshalJSON(data []byte) error {
	type alias Identity
	var id alias
	if err := json.Unmarshal(data, &id); err != nil {
		return fmt.Errorf("failed to unmarshal identity: %w", err)
	}
	*i = Identity(id)
	return i.Validate()
}
