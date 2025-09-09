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

package storage

import (
	"path"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"

	"github.com/fluxcd/pkg/apis/meta"
)

// LocalPath returns the secure local path of the given artifact
// (that is: relative to the Storage.BasePath).
func (s Storage) LocalPath(artifact meta.Artifact) string {
	if artifact.Path == "" {
		return ""
	}
	p, err := securejoin.SecureJoin(s.BasePath, artifact.Path)
	if err != nil {
		return ""
	}
	return p
}

// ArtifactDir returns the artifact dir path in the form of
// '<kind>/<namespace>/<name>'.
func ArtifactDir(kind, namespace, name string) string {
	kind = strings.ToLower(kind)
	return path.Join(kind, namespace, name)
}

// ArtifactPath returns the artifact path in the form of
// '<kind>/<namespace>/name>/<filename>'.
func ArtifactPath(kind, namespace, name, filename string) string {
	return path.Join(ArtifactDir(kind, namespace, name), filename)
}
