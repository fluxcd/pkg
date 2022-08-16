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

package patch

import (
	"fmt"
	"os"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/fluxcd/pkg/runtime/conditions/testdata"
	"github.com/fluxcd/pkg/runtime/testenv"
)

const (
	timeout         = time.Second * 10
	extendedTimeout = time.Second * 15
)

var (
	env *testenv.Environment
	ctx = ctrl.SetupSignalHandler()
)

func TestMain(m *testing.M) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(testdata.AddFakeToScheme(scheme))

	env = testenv.New(
		testenv.WithScheme(scheme),
		testenv.WithCRDPath("../conditions/testdata/crds"),
	)

	go func() {
		fmt.Println("Starting the test environment")
		if err := env.Start(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
		}
	}()
	<-env.Manager.Elected()

	code := m.Run()

	fmt.Println("Stopping the test environment")
	if err := env.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop the test environment: %v", err))
	}

	os.Exit(code)
}
