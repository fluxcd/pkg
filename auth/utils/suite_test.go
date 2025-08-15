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

package utils_test

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var scheme *runtime.Scheme

func init() {
	scheme = runtime.NewScheme()
	corev1.AddToScheme(scheme)
}

// newTestClient creates a fake Kubernetes client for testing.
// Use this for simple unit tests that don't require actual Kubernetes API calls.
// For tests that need TokenRequest API or other Kubernetes features, use envClient from newTestEnv instead.
func newTestClient() client.Client {
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}
