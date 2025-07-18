/*
Copyright 2024 The Flux authors

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

package chartutil

import (
	"sort"

	goyaml "go.yaml.in/yaml/v2"
)

func sortSlice(s []interface{}) {
	for _, item := range s {
		if nestedMS, ok := item.(goyaml.MapSlice); ok {
			SortMapSlice(nestedMS)
		} else if nestedSlice, ok := item.([]interface{}); ok {
			sortSlice(nestedSlice)
		}
	}
}

// SortMapSlice recursively sorts the given goyaml.MapSlice by key.
// It can be used in combination with Encode to sort YAML by key
// before encoding it.
func SortMapSlice(ms goyaml.MapSlice) {
	sort.Slice(ms, func(i, j int) bool {
		return ms[i].Key.(string) < ms[j].Key.(string)
	})

	for _, item := range ms {
		if nestedMS, ok := item.Value.(goyaml.MapSlice); ok {
			SortMapSlice(nestedMS)
		} else if nestedSlice, ok := item.Value.([]interface{}); ok {
			sortSlice(nestedSlice)
		}
	}
}
