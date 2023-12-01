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

package jsondiff

import (
	"context"
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/fluxcd/pkg/ssa/utils"
)

var (
	testEnv *envtest.Environment

	testClient client.Client
)

func TestMain(m *testing.M) {
	testEnv = &envtest.Environment{}

	fmt.Println("Starting the test environment")
	if _, err := testEnv.Start(); err != nil {
		panic(fmt.Sprintf("Failed to start the test environment: %v", err))
	}

	c, err := client.New(testEnv.Config, client.Options{})
	if err != nil {
		panic(fmt.Sprintf("Failed to create the client: %v", err))
	}
	testClient = c

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}
	os.Exit(code)
}

// CreateNamespace creates a namespace with the given generateName.
func CreateNamespace(ctx context.Context, generateName string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", generateName),
		},
	}
	if err := testClient.Create(ctx, ns); err != nil {
		return nil, err
	}
	return ns, nil
}

// LoadResource loads an unstructured.Unstructured resource from a file.
func LoadResource(p string) (*unstructured.Unstructured, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return utils.ReadObject(f)
}
