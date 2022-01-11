//go:build gofuzz
// +build gofuzz

/*
Copyright 2021 The Flux authors

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
package gitutil

import "errors"

// FuzzLibGit2Error implements a fuzzer that targets gitutil.LibGit2Error.
func FuzzLibGit2Error(data []byte) int {
	err := errors.New(string(data))
	_ = LibGit2Error(err)
	return 1
}
