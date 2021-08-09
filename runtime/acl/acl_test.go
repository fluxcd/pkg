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

package acl

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/pkg/apis/acl"
	"github.com/fluxcd/pkg/runtime/testenv"
)

func TestAclAuthorization(t *testing.T) {
	ctx := ctrl.SetupSignalHandler()
	testEnv := testenv.New()
	go func() {
		if err := testEnv.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the testenv: %v", err))
		}
	}()
	defer testEnv.Stop()

	aclAuth := NewAuthorization(testEnv.Client)

	tests := []struct {
		name         string
		namespace    *corev1.Namespace
		object       *corev1.ConfigMap
		reference    types.NamespacedName
		referenceAcl *acl.AccessFrom
		wantErr      bool
	}{
		{
			name:         "When in same namespace it ignores the ACL and grands access",
			object:       getObject("test1", "test1"),
			reference:    getReference("test1", "test1"),
			namespace:    getNamespaceWithLabels("test1", map[string]string{"tenant": "a"}),
			referenceAcl: getReferenceAcl(map[string]string{"tenant": "b"}),
			wantErr:      false,
		},
		{
			name:         "When in different namespace with nil ACL it denies access",
			object:       getObject("test2", "test2"),
			reference:    getReference("test2", "default"),
			namespace:    getNamespaceWithLabels("test2", nil),
			referenceAcl: nil,
			wantErr:      true,
		},
		{
			name:         "When in different namespace with empty ACL it denies access",
			object:       getObject("test3", "test3"),
			reference:    getReference("test3", "default"),
			namespace:    getNamespaceWithLabels("test3", nil),
			referenceAcl: &acl.AccessFrom{},
			wantErr:      true,
		},
		{
			name:         "When in different namespace with empty match labels it grants access",
			object:       getObject("test4", "test4"),
			reference:    getReference("test4", "default"),
			namespace:    getNamespaceWithLabels("test4", nil),
			referenceAcl: getReferenceAcl(nil),
			wantErr:      false,
		},
		{
			name:         "When in different namespace with matching labels it grands access",
			object:       getObject("test5", "test5"),
			reference:    getReference("test5", "default"),
			namespace:    getNamespaceWithLabels("test5", map[string]string{"tenant": "a"}),
			referenceAcl: getReferenceAcl(map[string]string{"tenant": "a"}),
			wantErr:      false,
		},
		{
			name:         "When in different namespace with mismatching labels it denies access",
			object:       getObject("test6", "test6"),
			reference:    getReference("test6", "default"),
			namespace:    getNamespaceWithLabels("test6", map[string]string{"tenant": "a"}),
			referenceAcl: getReferenceAcl(map[string]string{"tenant": "b"}),
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(testEnv.CreateAndWait(ctx, tt.namespace)).ToNot(HaveOccurred())

			hasAccess, err := aclAuth.HasAccessToRef(ctx, tt.object, tt.reference, tt.referenceAcl)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(hasAccess).To(BeFalse())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(hasAccess).To(BeTrue())
		})
	}
}

func getNamespaceWithLabels(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func getObject(name, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

func getReference(name, namespace string) types.NamespacedName {
	return types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}
}

func getReferenceAcl(labels map[string]string) *acl.AccessFrom {
	return &acl.AccessFrom{
		NamespaceSelectors: []acl.NamespaceSelector{
			{
				MatchLabels: labels,
			},
		},
	}
}
