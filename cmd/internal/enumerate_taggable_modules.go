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

package internal

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// EnumerateTaggableModules traverses the current directory and returns the
// paths of directories containing Go modules that are taggable for release.
func EnumerateTaggableModules() ([]string, error) {
	var nonTaggables = append([]string{"cmd"}, testModules...)
	var taggables []string
	err := fs.WalkDir(os.DirFS("."), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		for _, nonTaggable := range nonTaggables {
			if path == nonTaggable || strings.HasPrefix(path, nonTaggable+"/") {
				return nil
			}
		}
		f, err := os.Open(filepath.Join(path, "go.mod"))
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		f.Close()
		taggables = append(taggables, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(taggables)
	return taggables, nil
}
