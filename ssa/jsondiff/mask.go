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

package jsondiff

import (
	"github.com/wI2L/jsondiff"
	"strings"
)

const (
	sensitiveMaskDefault = "***"
	sensitiveMaskBefore  = "*** (before)"
	sensitiveMaskAfter   = "*** (after)"
)

// MaskSecretPatchData masks the data and stringData fields of a Secret object
// in the given JSON patch. It replaces the values with a default mask value if
// the field is added or removed. Otherwise, it replaces the values with a
// before/after mask value if the field is modified.
func MaskSecretPatchData(patch jsondiff.Patch) jsondiff.Patch {
	for i := range patch {
		v := &patch[i]
		oldMaskValue, newMaskValue := sensitiveMaskDefault, sensitiveMaskDefault

		if v.OldValue != nil && v.Value != nil {
			oldMaskValue = sensitiveMaskBefore
			newMaskValue = sensitiveMaskAfter
		}

		switch {
		case v.Path == "/data" || v.Path == "/stringData":
			maskMap(v.OldValue, v.Value)
		case strings.HasPrefix(v.Path, "/data/") || strings.HasPrefix(v.Path, "/stringData/"):
			if v.OldValue != nil {
				v.OldValue = oldMaskValue
			}
			if v.Value != nil {
				v.Value = newMaskValue
			}
		}
	}
	return patch
}

// maskMap replaces the values with a default mask value if a field is added or
// removed. Otherwise, it replaces the values with a before/after mask value if
// the field is modified.
func maskMap(from interface{}, to interface{}) {
	fromMap, fromIsMap := from.(map[string]interface{})
	if !fromIsMap || fromMap == nil {
		fromMap = make(map[string]interface{})
	}

	toMap, toIsMap := to.(map[string]interface{})
	if !toIsMap || toMap == nil {
		toMap = make(map[string]interface{})
	}

	for k := range fromMap {
		if _, ok := toMap[k]; ok {
			if fromMap[k] != toMap[k] {
				fromMap[k] = sensitiveMaskBefore
				toMap[k] = sensitiveMaskAfter
				continue
			}
		}
		fromMap[k] = sensitiveMaskDefault
	}
	for k := range toMap {
		if _, ok := fromMap[k]; !ok {
			toMap[k] = sensitiveMaskDefault
		}
	}
}
