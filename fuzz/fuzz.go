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
package fuzzing

import (
	"bytes"
	"encoding/json"
	"errors"
	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/fluxcd/pkg/gitutil"
	"github.com/fluxcd/pkg/runtime/events"
	"github.com/fluxcd/pkg/untar"
	"io"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"net/http/httptest"
	"os"
)

// FuzzUntar implements a fuzzer that
// targets untar.Untar()
func FuzzUntar(data []byte) int {
	r := bytes.NewReader(data)
	tmpDir, err := os.MkdirTemp("", "dir-")
	if err != nil {
		return 0
	}
	defer os.RemoveAll(tmpDir)
	_, _ = untar.Untar(r, tmpDir)
	return 1
}

// FuzzLibGit2Error implements a fuzzer that
// targets gitutil.LibGit2Error
func FuzzLibGit2Error(data []byte) int {
	err := errors.New(string(data))
	_ = gitutil.LibGit2Error(err)
	return 1
}

// FuzzEventInfof implements a fuzzer that
// targets eventRecorder.EventInfof()
func FuzzEventInfof(data []byte) int {
	f := fuzz.NewConsumer(data)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}

		var payload events.Event
		err = json.Unmarshal(b, &payload)
		if err != nil {
			return
		}
	}))
	defer ts.Close()
	eventRecorder, err := events.NewRecorder(ts.URL, "test-controller")
	if err != nil {
		return 0
	}
	eventRecorder.Client.RetryMax = 2
	obj := corev1.ObjectReference{}
	err = f.GenerateStruct(&obj)
	if err != nil {
		return 0
	}
	severity, err := f.GetString()
	if err != nil {
		return 0
	}
	reason, err := f.GetString()
	if err != nil {
		return 0
	}
	_ = eventRecorder.EventInfof(obj, nil, severity, reason, obj.Name)
	return 1
}
