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

package events

import (
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/pkg/runtime/testenv"
)

var (
	env *testenv.Environment
	ctx = ctrl.SetupSignalHandler()
)

func TestMain(m *testing.M) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))

	env = testenv.New(
		testenv.WithScheme(scheme),
	)

	go func() {
		fmt.Println("Starting the test environment")
		if err := env.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
		}
	}()
	<-env.Manager.Elected()

	// Create test namespace.
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gitops-system",
		},
	}
	if err := env.Client.Create(ctx, ns); err != nil {
		panic(fmt.Sprintf("Failed to create gitops-system namespace: %v", err))
	}

	code := m.Run()

	if err := env.Client.Delete(ctx, ns); err != nil {
		panic(fmt.Sprintf("Failed to delete gitops-system namespace: %v", err))
	}

	fmt.Println("Stopping the test environment")
	if err := env.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}
