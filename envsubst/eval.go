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

// Forked from: https://github.com/drone/envsubst
// MIT License
// Copyright (c) 2017 drone.io.

package envsubst

import "os"

// Eval replaces ${var} in the string based on the mapping function.
func Eval(s string, mapping func(string) (string, bool)) (string, error) {
	t, err := Parse(s)
	if err != nil {
		return s, err
	}
	return t.Execute(mapping)
}

// EvalEnv replaces ${var} in the string according to the values of the
// current environment variables. References to undefined variables are
// replaced by the empty string.
func EvalEnv(s string, strict bool) (string, error) {
	mapping := Getenv
	if strict {
		mapping = os.LookupEnv
	}
	return Eval(s, mapping)
}

func Getenv(s string) (string, bool) {
	return os.Getenv(s), true
}
