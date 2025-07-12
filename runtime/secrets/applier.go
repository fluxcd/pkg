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

package secrets

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplyOptions configures the Kubernetes secret apply operations.
type ApplyOptions struct {
	owner       string
	labels      map[string]string
	annotations map[string]string
	immutable   *bool
	force       bool
}

// ApplyOption configures an ApplyOptions.
type ApplyOption func(*ApplyOptions)

// WithOwner sets the field owner for server-side apply.
func WithOwner(owner string) ApplyOption {
	return func(o *ApplyOptions) {
		o.owner = owner
	}
}

// WithLabels sets labels to be applied to the secret.
func WithLabels(labels map[string]string) ApplyOption {
	return func(o *ApplyOptions) {
		o.labels = labels
	}
}

// WithAnnotations sets annotations to be applied to the secret.
func WithAnnotations(annotations map[string]string) ApplyOption {
	return func(o *ApplyOptions) {
		o.annotations = annotations
	}
}

// WithImmutable sets the immutable flag for the secret.
func WithImmutable(immutable bool) ApplyOption {
	return func(o *ApplyOptions) {
		o.immutable = &immutable
	}
}

// WithForce enables force apply, which can result in the deletion of existing
// secrets that are immutable or have a different type.
func WithForce() ApplyOption {
	return func(o *ApplyOptions) {
		o.force = true
	}
}

// Apply applies a Kubernetes secret using server-side apply with configurable options.
// If the secret already exists and is immutable, the object is deleted first.
func Apply(ctx context.Context, c client.Client, secret *corev1.Secret, opts ...ApplyOption) error {
	options := &ApplyOptions{}
	for _, opt := range opts {
		opt(options)
	}

	// Set labels from options.
	if options.labels != nil {
		if secret.Labels == nil {
			secret.Labels = make(map[string]string)
		}
		for k, v := range options.labels {
			secret.Labels[k] = v
		}
	}

	// Set annotations from options.
	if options.annotations != nil {
		if secret.Annotations == nil {
			secret.Annotations = make(map[string]string)
		}
		for k, v := range options.annotations {
			secret.Annotations[k] = v
		}
	}

	// Set the immutable flag from options.
	if options.immutable != nil {
		secret.Immutable = options.immutable
	}

	// Convert StringData to Data to ensure compatibility with server-side apply.
	if len(secret.StringData) > 0 {
		if secret.Data == nil {
			secret.Data = make(map[string][]byte, len(secret.StringData))
		}
		for k, v := range secret.StringData {
			secret.Data[k] = []byte(v)
		}
		secret.StringData = nil
	}

	// Ensure the secret has a type set, defaulting to Opaque if not specified.
	if secret.Type == "" {
		secret.Type = corev1.SecretTypeOpaque
	}

	// Check if the secret already exists.
	existing := &corev1.Secret{}
	mustDelete := false
	if err := c.Get(ctx, client.ObjectKeyFromObject(secret), existing); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("could not get existing secret: %w", err)
		}
	} else {
		// If the existing secret is immutable or has a different type, mark it for deletion.
		if (existing.Immutable != nil && *existing.Immutable) || existing.Type != secret.Type {
			mustDelete = true
		}
	}

	// Delete the existing secret if necessary.
	if mustDelete && options.force {
		if err := c.Delete(ctx, existing); err != nil {
			return fmt.Errorf("could not delete existing secret: %w", err)
		}
	}

	// Set the GroupVersionKind which is required for server-side apply.
	secret.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Secret"))

	// Apply the secret using server-side apply.
	patchOps := []client.PatchOption{
		client.ForceOwnership,
	}
	if options.owner != "" {
		patchOps = append(patchOps, client.FieldOwner(options.owner))
	}
	return c.Patch(ctx, secret, client.Apply, patchOps...)
}
