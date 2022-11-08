//go:build gofuzz
// +build gofuzz

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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"testing"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/fluxcd/pkg/runtime/testenv"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrl "sigs.k8s.io/controller-runtime"

	eventv1 "github.com/fluxcd/pkg/apis/event/v1beta1"
)

var (
	doOnce  sync.Once
	fuzzEnv *testenv.Environment
	fuzzTs  *httptest.Server
	fuzzCtx = ctrl.SetupSignalHandler()
)

const defaultBinVersion = "1.24"

// Fuzz_Eventf is locked behind a build tag, as its test setup conflicts with the
// suite_test.go. This test should be refactored to no longer require testenv,
// which will resolve the problem whilst making the test more effient.
//
// TODO: refactor and remove build tag.
func Fuzz_Eventf(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		doOnce.Do(func() {
			if err := ensureDependencies(); err != nil {
				panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
			}
		})

		fuzzTs = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				return
			}

			var payload eventv1.Event
			err = json.Unmarshal(b, &payload)
			if err != nil {
				return
			}
		}))
		defer fuzzTs.Close()

		scheme := runtime.NewScheme()
		utilruntime.Must(corev1.AddToScheme(scheme))

		fuzzEnv = testenv.New(
			testenv.WithScheme(scheme),
		)

		go func() {
			fmt.Println("Starting the test environment")
			if err := fuzzEnv.Start(fuzzCtx); err != nil {
				panic(fmt.Sprintf("Failed to start the test environment manager: %v", err))
			}
		}()
		<-fuzzEnv.Manager.Elected()

		eventRecorder, err := NewRecorder(fuzzEnv, ctrl.Log, fuzzTs.URL, "test-controller")
		if err != nil {
			return
		}
		eventRecorder.Client.RetryMax = 2

		f := fuzz.NewConsumer(data)
		obj := corev1.ConfigMap{}
		err = f.GenerateStruct(&obj)
		if err != nil {
			return
		}
		eventtype, err := f.GetString()
		if err != nil {
			return
		}
		reason, err := f.GetString()
		if err != nil {
			return
		}
		eventRecorder.Eventf(&obj, eventtype, reason, obj.Name)

		_ = fuzzEnv.Stop()
	})
}

func envtestBinVersion() string {
	if binVersion := os.Getenv("ENVTEST_BIN_VERSION"); binVersion != "" {
		return binVersion
	}
	return defaultBinVersion
}

func ensureDependencies() error {
	// only install dependencies when running inside a container
	if _, err := os.Stat("/.dockerenv"); os.IsNotExist(err) {
		return nil
	}

	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		binVersion := envtestBinVersion()
		cmd := exec.Command("/usr/bin/bash", "-c", fmt.Sprintf(`go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest && \
		/root/go/bin/setup-envtest use -p path %s`, binVersion))

		cmd.Env = append(os.Environ(), "GOPATH=/root/go")
		assetsPath, err := cmd.Output()
		if err == nil {
			os.Setenv("KUBEBUILDER_ASSETS", string(assetsPath))
		}
		return err
	}

	return nil
}
