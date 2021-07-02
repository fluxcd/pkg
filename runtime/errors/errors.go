/*
Copyright 2020 The Flux authors

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
	"fmt"

	"k8s.io/apimachinery/pkg/types"
)

// ReconciliationError is describes a generic reconciliation error for a resource, it includes the Kind and NamespacedName
// of the resource, and any underlying Err.
type ReconciliationError struct {
	Kind           string
	NamespacedName types.NamespacedName
	Err            error
}

func (e *ReconciliationError) Error() string {
	return fmt.Sprintf("%s '%s' reconciliation failed: %v", e.Kind, e.NamespacedName.String(), e.Err)
}

func (e *ReconciliationError) Unwrap() error {
	return e.Err
}

// ResourceNotReadyError describes an error in which a referred resource is not in a meta.ReadyCondition state,
// it includes the Kind and NamespacedName, and any underlying Err.
type ResourceNotReadyError struct {
	Kind           string
	NamespacedName types.NamespacedName
	Err            error
}

func (e *ResourceNotReadyError) Error() string {
	return fmt.Sprintf("%s resource '%s' is not ready", e.Kind, e.NamespacedName.String())
}

func (e *ResourceNotReadyError) Unwrap() error {
	return e.Err
}

// ResourceNotFoundError describes an error in which a referred resource could not be found,
// it includes the Kind and NamespacedName, and any underlying Err.
type ResourceNotFoundError struct {
	Kind           string
	NamespacedName types.NamespacedName
	Err            error
}

func (e *ResourceNotFoundError) Error() string {
	return fmt.Sprintf("%s resource '%s' could not be found", e.Kind, e.NamespacedName.String())
}

// UnsupportedResourceKindError describes an error in which a referred resource is of an unsupported kind,
// it includes the Kind and NamespacedName of the resource, and any underlying Err.
type UnsupportedResourceKindError struct {
	Kind           string
	NamespacedName types.NamespacedName
	SupportedKinds []string
}

func (e *UnsupportedResourceKindError) Error() string {
	err := fmt.Sprintf("source '%s' with kind %s is not supported", e.NamespacedName.String(), e.Kind)
	if len(e.SupportedKinds) == 0 {
		return err
	}
	return fmt.Sprintf("%s (must be one of: %q)", err, e.SupportedKinds)
}

// GarbageCollectionError is describes a garbage collection error for a resources, it includes the Kind and
// NamespacedName of the resource, and the underlying Err.
type GarbageCollectionError struct {
	Kind           string
	NamespacedName types.NamespacedName
	Err            error
}

func (e *GarbageCollectionError) Error() string {
	return fmt.Sprintf("failed to garbage collect %s '%s': %v", e.Kind, e.NamespacedName, e.Err)
}

func (e *GarbageCollectionError) Unwrap() error {
	return e.Err
}
