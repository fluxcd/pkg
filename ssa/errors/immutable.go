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

package errors

import (
	"regexp"

	"k8s.io/apimachinery/pkg/api/errors"
)

// Match CEL immutable error variants.
var matchImmutableFieldErrors = []*regexp.Regexp{
	regexp.MustCompile(`.*is\simmutable.*`),
	regexp.MustCompile(`.*immutable\sfield.*`),
}

// IsImmutableError checks if the given error is an immutable error.
func IsImmutableError(err error) bool {
	// Detect immutability like kubectl does
	// https://github.com/kubernetes/kubectl/blob/8165f83007/pkg/cmd/apply/patcher.go#L201
	if errors.IsConflict(err) || errors.IsInvalid(err) {
		return true
	}

	// Detect immutable errors returned by custom admission webhooks and Kubernetes CEL
	// https://kubernetes.io/blog/2022/09/29/enforce-immutability-using-cel/#immutablility-after-first-modification
	for _, fieldError := range matchImmutableFieldErrors {
		if fieldError.MatchString(err.Error()) {
			return true
		}
	}

	return false
}
