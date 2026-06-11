/*
Copyright 2026 The Flux authors

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

package client

import (
	"fmt"
	"os"
	"testing"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/fluxcd/pkg/runtime/testenv"
)

var (
	testEnv    *testenv.Environment
	ctx        = ctrl.SetupSignalHandler()
	kubeConfig []byte
)

func TestMain(m *testing.M) {
	testEnv = testenv.New()

	go func() {
		fmt.Println("Starting the test environment")
		if err := testEnv.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
		}
	}()
	<-testEnv.Manager.Elected()

	user, err := testEnv.AddUser(envtest.User{
		Name:   "testenv-admin",
		Groups: []string{"system:masters"},
	}, nil)
	if err != nil {
		panic(fmt.Sprintf("Failed to create testenv-admin user: %v", err))
	}

	kubeConfig, err = user.KubeConfig()
	if err != nil {
		panic(fmt.Sprintf("Failed to create the testenv-admin user kubeconfig: %v", err))
	}

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}
