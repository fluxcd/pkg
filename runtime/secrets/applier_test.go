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

package secrets_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/fluxcd/pkg/runtime/secrets"
)

func TestApply(t *testing.T) {
	namespace, err := env.CreateNamespace(context.Background(), "test-apply")
	if err != nil {
		t.Fatalf("failed to create test namespace: %v", err)
	}
	ns := namespace.Name
	immutable := true

	tests := []struct {
		name           string
		secret         *corev1.Secret
		existingSecret *corev1.Secret
		expectError    bool
		errMsg         string
		validateFunc   func(g *WithT, secret *corev1.Secret)
	}{
		{
			name: "apply new secret with string data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			validateFunc: func(g *WithT, s *corev1.Secret) {
				g.Expect(s.StringData).To(BeNil())
				g.Expect(s.Data).To(HaveLen(2))
				g.Expect(s.Data["key1"]).To(Equal([]byte("value1")))
				g.Expect(s.Data["key2"]).To(Equal([]byte("value2")))
			},
		},
		{
			name: "apply new secret with both string data and data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-mixed",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"existing": []byte("data"),
				},
				StringData: map[string]string{
					"new": "string-data",
				},
			},
			validateFunc: func(g *WithT, s *corev1.Secret) {
				g.Expect(s.StringData).To(BeNil())
				g.Expect(s.Data).To(HaveLen(2))
				g.Expect(s.Data["existing"]).To(Equal([]byte("data")))
				g.Expect(s.Data["new"]).To(Equal([]byte("string-data")))
			},
		},
		{
			name: "apply secret with only data (no string data)",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-data-only",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			},
			validateFunc: func(g *WithT, s *corev1.Secret) {
				g.Expect(s.StringData).To(BeNil())
				g.Expect(s.Data).To(HaveLen(1))
				g.Expect(s.Data["key"]).To(Equal([]byte("value")))
			},
		},
		{
			name: "apply secret that merges existing mutable secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-mutable",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"test1": "new",
				},
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-mutable",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"test1": []byte("old"),
					"test2": []byte("old"),
				},
				Immutable: nil, // mutable secret
			},
			validateFunc: func(g *WithT, s *corev1.Secret) {
				g.Expect(s.Data).To(HaveLen(2))
				g.Expect(s.Data["test1"]).To(Equal([]byte("new")))
				g.Expect(s.Data["test2"]).To(Equal([]byte("old")))
			},
		},
		{
			name: "apply secret that replaces existing immutable secret",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-immutable",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"new": "data",
				},
				Immutable: &immutable,
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-immutable",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"old": []byte("immutable-data"),
				},
				Immutable: &immutable,
			},
			validateFunc: func(g *WithT, s *corev1.Secret) {
				g.Expect(s.Data).To(HaveLen(1))
				g.Expect(s.Data["new"]).To(Equal([]byte("data")))
			},
		},
		{
			name: "apply secret that replaces existing secret with different type",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-type",
					Namespace: ns,
				},
				Type: corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					"username": []byte("val"),
					"password": []byte("val"),
				},
				Immutable: &immutable,
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-type",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"old": []byte("data"),
				},
			},
			validateFunc: func(g *WithT, s *corev1.Secret) {
				g.Expect(s.Data).To(HaveLen(2))
				g.Expect(s.Data["username"]).To(Equal([]byte("val")))
				g.Expect(s.Data["password"]).To(Equal([]byte("val")))
			},
		},
		{
			name: "fails to apply immutable secret with different data",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-immutable-fail",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"new": "data",
				},
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-immutable-fail",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				Data: map[string][]byte{
					"old": []byte("immutable-data"),
				},
				Immutable: &immutable,
			},
			expectError: true,
			errMsg:      "field is immutable",
		},
		{
			name: "fails to apply secret with different type",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-auth-fail",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"new": "data",
				},
			},
			existingSecret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existing-auth-fail",
					Namespace: ns,
				},
				Type: corev1.SecretTypeBasicAuth,
				Data: map[string][]byte{
					"username": []byte("val"),
					"password": []byte("val"),
				},
			},
			expectError: true,
			errMsg:      "field is immutable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create the existing secret if provided
			if tt.existingSecret != nil {
				err := env.Create(context.Background(), tt.existingSecret)
				g.Expect(err).To(Not(HaveOccurred()))
				g.Eventually(func() error {
					s := &corev1.Secret{}
					return env.Get(context.Background(), client.ObjectKeyFromObject(tt.existingSecret), s)
				}, timeout).ShouldNot(HaveOccurred())
			}

			// Set the options for applying the secret
			opts := []secrets.ApplyOption{
				secrets.WithOwner("test-owner"),
			}
			if tt.secret.Immutable != nil {
				opts = append(opts, secrets.WithImmutable(*tt.secret.Immutable), secrets.WithForce())
			}

			// Apply the secret
			err = secrets.Apply(context.Background(), env, tt.secret, opts...)

			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
				if tt.errMsg != "" {
					g.Expect(err.Error()).To(ContainSubstring(tt.errMsg))
				}
				return
			}

			g.Expect(err).To(Not(HaveOccurred()))

			// Verify the secret was applied correctly
			appliedSecret := &corev1.Secret{}
			g.Eventually(func() error {
				return env.Get(context.Background(), client.ObjectKeyFromObject(tt.secret), appliedSecret)
			}, timeout).ShouldNot(HaveOccurred())

			// Run validation function on the applied secret
			if tt.validateFunc != nil {
				tt.validateFunc(g, appliedSecret)
			}
		})
	}
}

func TestApply_Options(t *testing.T) {
	namespace, err := env.CreateNamespace(context.Background(), "test-options")
	if err != nil {
		t.Fatalf("failed to create test namespace: %v", err)
	}
	ns := namespace.Name

	tests := []struct {
		name         string
		secret       *corev1.Secret
		options      []secrets.ApplyOption
		validateFunc func(g *WithT, secret *corev1.Secret)
	}{
		{
			name: "apply secret with labels",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-labels",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"key": "value",
				},
			},
			options: []secrets.ApplyOption{
				secrets.WithOwner("test-owner"),
				secrets.WithLabels(map[string]string{
					"app":     "my-app",
					"version": "v1.0.0",
				}),
			},
			validateFunc: func(g *WithT, secret *corev1.Secret) {
				g.Expect(secret.Labels).To(HaveLen(2))
				g.Expect(secret.Labels["app"]).To(Equal("my-app"))
				g.Expect(secret.Labels["version"]).To(Equal("v1.0.0"))
			},
		},
		{
			name: "apply secret with annotations",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-annotations",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"key": "value",
				},
			},
			options: []secrets.ApplyOption{
				secrets.WithOwner("test-owner"),
				secrets.WithAnnotations(map[string]string{
					"created-by": "automation",
					"purpose":    "testing",
				}),
			},
			validateFunc: func(g *WithT, secret *corev1.Secret) {
				g.Expect(secret.Annotations).To(HaveLen(2))
				g.Expect(secret.Annotations["created-by"]).To(Equal("automation"))
				g.Expect(secret.Annotations["purpose"]).To(Equal("testing"))
			},
		},
		{
			name: "apply secret with immutable flag",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-immutable",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"key": "value",
				},
			},
			options: []secrets.ApplyOption{
				secrets.WithOwner("test-owner"),
				secrets.WithImmutable(true),
			},
			validateFunc: func(g *WithT, secret *corev1.Secret) {
				g.Expect(secret.Immutable).ToNot(BeNil())
				g.Expect(*secret.Immutable).To(BeTrue())
			},
		},
		{
			name: "apply secret with all options",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-all-options",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"username": "admin",
					"password": "secret",
				},
			},
			options: []secrets.ApplyOption{
				secrets.WithOwner("custom-owner"),
				secrets.WithLabels(map[string]string{
					"app":        "test-app",
					"component":  "auth",
					"managed-by": "automation",
				}),
				secrets.WithAnnotations(map[string]string{
					"description":  "Test secret with all options",
					"created-at":   "2025-01-01",
					"last-updated": "2025-01-01",
				}),
				secrets.WithImmutable(true),
			},
			validateFunc: func(g *WithT, secret *corev1.Secret) {
				// Validate labels
				g.Expect(secret.Labels).To(HaveLen(3))
				g.Expect(secret.Labels["app"]).To(Equal("test-app"))
				g.Expect(secret.Labels["component"]).To(Equal("auth"))
				g.Expect(secret.Labels["managed-by"]).To(Equal("automation"))

				// Validate annotations
				g.Expect(secret.Annotations).To(HaveLen(3))
				g.Expect(secret.Annotations["description"]).To(Equal("Test secret with all options"))
				g.Expect(secret.Annotations["created-at"]).To(Equal("2025-01-01"))
				g.Expect(secret.Annotations["last-updated"]).To(Equal("2025-01-01"))

				// Validate immutable flag
				g.Expect(secret.Immutable).ToNot(BeNil())
				g.Expect(*secret.Immutable).To(BeTrue())

				// Validate data
				g.Expect(secret.Data).To(HaveLen(2))
				g.Expect(secret.Data["username"]).To(Equal([]byte("admin")))
				g.Expect(secret.Data["password"]).To(Equal([]byte("secret")))
			},
		},
		{
			name: "apply secret with existing labels and annotations (merge)",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-merge",
					Namespace: ns,
					Labels: map[string]string{
						"existing-label": "existing-value",
					},
					Annotations: map[string]string{
						"existing-annotation": "existing-value",
					},
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"key": "value",
				},
			},
			options: []secrets.ApplyOption{
				secrets.WithOwner("test-owner"),
				secrets.WithLabels(map[string]string{
					"new-label": "new-value",
				}),
				secrets.WithAnnotations(map[string]string{
					"new-annotation": "new-value",
				}),
			},
			validateFunc: func(g *WithT, secret *corev1.Secret) {
				// Validate merged labels
				g.Expect(secret.Labels).To(HaveLen(2))
				g.Expect(secret.Labels["existing-label"]).To(Equal("existing-value"))
				g.Expect(secret.Labels["new-label"]).To(Equal("new-value"))

				// Validate merged annotations
				g.Expect(secret.Annotations).To(HaveLen(2))
				g.Expect(secret.Annotations["existing-annotation"]).To(Equal("existing-value"))
				g.Expect(secret.Annotations["new-annotation"]).To(Equal("new-value"))
			},
		},
		{
			name: "apply secret with no options (default behavior)",
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-no-options",
					Namespace: ns,
				},
				Type: corev1.SecretTypeOpaque,
				StringData: map[string]string{
					"key": "value",
				},
			},
			options: []secrets.ApplyOption{
				secrets.WithOwner("test-owner"), // Owner is required for server-side apply
			},
			validateFunc: func(g *WithT, secret *corev1.Secret) {
				// Should have no labels or annotations from options
				g.Expect(secret.Labels).To(BeNil())
				g.Expect(secret.Annotations).To(BeNil())
				g.Expect(secret.Immutable).To(BeNil())
				// Data should still be converted
				g.Expect(secret.Data).To(HaveLen(1))
				g.Expect(secret.Data["key"]).To(Equal([]byte("value")))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Apply the secret with options
			err := secrets.Apply(context.Background(), env, tt.secret, tt.options...)

			g.Expect(err).To(Not(HaveOccurred()))

			// Verify the secret was applied correctly
			appliedSecret := &corev1.Secret{}
			g.Eventually(func() error {
				return env.Get(context.Background(), client.ObjectKeyFromObject(tt.secret), appliedSecret)
			}, timeout).ShouldNot(HaveOccurred())

			// Run validation function if provided (validate both input transformation and cluster state)
			if tt.validateFunc != nil {
				tt.validateFunc(g, tt.secret)     // Validate input secret transformation
				tt.validateFunc(g, appliedSecret) // Validate cluster secret state
			}
		})
	}
}
