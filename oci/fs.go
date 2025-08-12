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

package oci

import "github.com/fluxcd/pkg/oci/internal/fs"

// RenameWithFallback attempts to rename a file or directory, but falls back to
// copying in the event of a cross-device link error. If the fallback copy
// succeeds, src is still removed, emulating normal rename behavior.
func RenameWithFallback(src, dst string) error {
	return fs.RenameWithFallback(src, dst)
}
