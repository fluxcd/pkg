/*
Copyright 2022 The Flux authors

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

package patch

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SerialPatcher provides serial patching of object using the patch helper. It
// remembers the state of the last patched object and uses that to calculate
// the patch aginst a new object.
type SerialPatcher struct {
	client       client.Client
	beforeObject client.Object
}

// NewSerialPatcher returns a SerialPatcher with the given object as the initial
// base object for the patching operations.
func NewSerialPatcher(obj client.Object, c client.Client) *SerialPatcher {
	return &SerialPatcher{
		client:       c,
		beforeObject: obj.DeepCopyObject().(client.Object),
	}
}

// Patch performs patching operation of the SerialPatcher and updates the
// beforeObject after a successful patch for subsequent patching.
func (sp *SerialPatcher) Patch(ctx context.Context, obj client.Object, options ...Option) error {
	// Create a new patch helper with the before object.
	patcher, err := NewHelper(sp.beforeObject, sp.client)
	if err != nil {
		return err
	}

	// Patch with the changes from the new object.
	if err := patcher.Patch(ctx, obj, options...); err != nil {
		return err
	}

	// Update the before object for next patch.
	sp.beforeObject = obj.DeepCopyObject().(client.Object)

	return nil
}
