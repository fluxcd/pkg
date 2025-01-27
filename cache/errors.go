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

package cache

import (
	"errors"
	"fmt"
)

// CacheErrorReason is a type that represents the reason for a cache error.
type CacheErrorReason struct {
	reason string
	msg    string
}

// Error gives a human-readable description of the error.
func (e CacheErrorReason) Error() string {
	return e.msg
}

type CacheError struct {
	Reason CacheErrorReason
	Err    error
}

// Error returns Err as a string, prefixed with the Reason to provide context.
func (e *CacheError) Error() string {
	if e.Reason.Error() == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %s", e.Reason.Error(), e.Err.Error())
}

// Is returns true if the Reason or Err equals target.
// It can be used to programmatically place an arbitrary Err in the
// context of the Cache:
//
//	err := &CacheError{Reason: ErrCacheFull, Err: errors.New("arbitrary resize error")}
//	errors.Is(err, ErrCacheFull)
func (e *CacheError) Is(target error) bool {
	if e.Reason == target {
		return true
	}
	return errors.Is(e.Err, target)
}

// Unwrap returns the underlying Err.
func (e *CacheError) Unwrap() error {
	return e.Err
}

var (
	ErrNotFound    = CacheErrorReason{"NotFound", "object not found"}
	ErrCacheClosed = CacheErrorReason{"CacheClosed", "cache is closed"}
	ErrCacheFull   = CacheErrorReason{"CacheFull", "cache is full"}
	ErrInvalidSize = CacheErrorReason{"InvalidSize", "invalid size"}
)
