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

package serviceaccounttoken

import (
	"encoding/json"
	"fmt"
)

// Identity represents an Azure identity that can be impersonated.
type Identity struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// String implements auth.Identity.
func (i *Identity) String() string {
	return fmt.Sprintf("system:serviceaccount:%s:%s", i.Namespace, i.Name)
}

// UnmarshalJSON implements json.Unmarshaler to validate the identity format when unmarshaling.
func (i *Identity) UnmarshalJSON(data []byte) error {
	type alias Identity
	var id alias
	if err := json.Unmarshal(data, &id); err != nil {
		return fmt.Errorf("failed to unmarshal identity: %w", err)
	}
	if id.Name == "" {
		return fmt.Errorf("name is required")
	}
	if id.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	*i = Identity(id)
	return nil
}
