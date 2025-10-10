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

package controller

import (
	"context"
	"strings"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// controllerBuilder wraps a *builder.Builder to
// enhance it with additional functionality.
type controllerBuilder struct {
	*builder.Builder
	mgr        manager.Manager
	reconciler *reconcilerWrapper
}

// NewControllerManagedBy returns a wrapped *builder.Builder
// that facilitates building a controller for a specific
// object type harvesting the capabilities of the reconciler
// wrapper.
func NewControllerManagedBy(mgr manager.Manager, r *reconcilerWrapper) *controllerBuilder {
	return &controllerBuilder{
		Builder:    ctrl.NewControllerManagedBy(mgr),
		mgr:        mgr,
		reconciler: r,
	}
}

// For is similar to builder.Builder.For, but internally
// uses WatchesRawSource to set up the watch harvesting
// the capabilities of the reconciler wrapper.
func (c *controllerBuilder) For(obj client.Object, pred predicate.Predicate) *controllerBuilder {
	// Do the same as builder.Builder.For to define the controller name,
	// lowercased kind of the object being watched.
	gvk, err := apiutil.GVKForObject(obj, c.mgr.GetScheme())
	// Here we need to panic because builder.Builder.For does not return an error.
	// This panic is fine, as it is caught during the controller initialization.
	if err != nil {
		panic(err)
	}
	name := strings.ToLower(gvk.Kind)

	c.Named(name)
	c.WatchesRawSource(source.Kind(
		c.mgr.GetCache(),
		obj,
		c.reconciler.EnqueueRequestsFromMapFunc(gvk.Kind, func(ctx context.Context, obj client.Object) []ctrl.Request {
			return []ctrl.Request{{NamespacedName: client.ObjectKeyFromObject(obj)}}
		}),
		pred,
	))
	return c
}
