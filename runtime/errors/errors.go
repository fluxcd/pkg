/*
Copyright 2020 The Flux CD contributors.

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

// ReconciliationError is returned on a reconciliation failure for a resource,
// it includes the Kind and NamespacedName of the resource the reconciliation
// was performed for, and the underlying Err.
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

// SourceNotReadyError is returned when a source is not in a ready condition
// during a reconciliation attempt, it includes the Kind and NamespacedName of
// the source.
type SourceNotReadyError struct {
	Kind           string
	NamespacedName types.NamespacedName
}

func (e *SourceNotReadyError) Error() string {
	return fmt.Sprintf("%s source '%s' is not ready", e.Kind, e.NamespacedName.String())
}

// SourceNotFoundError is returned if a referred source was not found, it
// includes the Kind and NamespacedName of the source.
type SourceNotFoundError struct {
	Kind           string
	NamespacedName types.NamespacedName
}

func (e *SourceNotFoundError) Error() string {
	return fmt.Sprintf("%s source '%s' does not exist", e.Kind, e.NamespacedName.String())
}

// UnsupportedSourceKindError is returned if a referred source is of an
// unsupported kind, it includes the Kind and Namespace of the source, and MAY
// contain a string slice of SupportedKinds.
type UnsupportedSourceKindError struct {
	Kind           string
	NamespacedName types.NamespacedName
	SupportedKinds []string
}

func (e *UnsupportedSourceKindError) Error() string {
	err := fmt.Sprintf("source '%s' with kind %s is not supported", e.NamespacedName.String(), e.Kind)
	if len(e.SupportedKinds) == 0 {
		return err
	}
	return fmt.Sprintf("%s (must be one of: %q)", err, e.SupportedKinds)
}

// ArtifactAcquisitionError is returned if the artifact of a source could not be
// acquired, it includes the Kind and NamespacedName of the source that
// advertised the artifact, and MAY contain an underlying Err.
type ArtifactAcquisitionError struct {
	Kind           string
	NamespacedName types.NamespacedName
	Err            error
}

func (e *ArtifactAcquisitionError) Error() string {
	err := fmt.Sprintf("failed to acquire %s artifact from '%s'", e.Kind, e.NamespacedName.String())
	if e.Err == nil {
		return err
	}
	return fmt.Sprintf("%s: %v", err, e.Err)
}

func (e *ArtifactAcquisitionError) Unwrap() error {
	return e.Err
}

// DependencyNotReadyError is returned if a referred dependency resource is not
// in a ready condition, it includes the Kind and NamespacedName of the
// dependency.
type DependencyNotReadyError struct {
	Kind           string
	NamespacedName types.NamespacedName
}

func (e *DependencyNotReadyError) Error() string {
	return fmt.Sprintf("dependency '%s' of kind %s is not ready", e.NamespacedName.String(), e.Kind)
}

// DependencyNotFoundError is returned if a referred dependency resource was not
// found, it includes the Kind and NamespacedName of the dependency.
type DependencyNotFoundError struct {
	Kind           string
	NamespacedName types.NamespacedName
}

func (e *DependencyNotFoundError) Error() string {
	return fmt.Sprintf("dependency '%s' of kind %s does not exist", e.NamespacedName, e.Kind)
}

// GarbageCollectionError is returned on a garbage collection failure for a
// resource, it includes the Kind and NamespacedName the garbage collection
// failed for, and the underlying Err.
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
