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

// EnumerateBumpableModules returns a list of Go modules that can be bumped
// in the github.com/fluxcd/pkg repository. This includes all modules that
// are taggable, as well as specific modules that are not tagged but should
// still be considered for version bumps.
func EnumerateBumpableModules(taggable []string) []string {
	return append(taggable, testModules...)
}
