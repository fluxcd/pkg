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

package controller_test

import (
	"os"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/spf13/pflag"
	"golang.org/x/net/http/httpproxy"

	"github.com/fluxcd/pkg/runtime/controller"
)

func Test_ConnectionOptions_BindFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "empty flag sets default value",
			args:     []string{""},
			expected: true,
		},
		{
			name:     "--insecure-allow-http set to false",
			args:     []string{"--insecure-allow-http=false"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			f := pflag.NewFlagSet("test", pflag.ContinueOnError)
			opts := controller.ConnectionOptions{}
			opts.BindFlags(f)

			err := f.Parse(tt.args)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(opts.AllowHTTP).To(Equal(tt.expected))
		})
	}
}

func Test_ConnectionOptions_CheckEnvironmentCompatibility(t *testing.T) {
	tests := []struct {
		name        string
		allowHTTP   bool
		beforeFunc  func()
		wantErrMsg  string
		cleanupFunc func()
	}{
		{
			name:      "http proxy set and http conns blocked",
			allowHTTP: false,
			beforeFunc: func() {
				os.Setenv("HTTP_PROXY", "http://example.com")
			},
			wantErrMsg: "use of insecure plain HTTP connections is blocked: found HTTP proxy set in environment",
			cleanupFunc: func() {
				os.Unsetenv("HTTP_PROXY")
			},
		},
		{
			name:      "https proxy set with http url and http conns blocked",
			allowHTTP: false,
			beforeFunc: func() {
				os.Setenv("HTTPS_PROXY", "http://example.com")
			},
			wantErrMsg: "use of insecure plain HTTP connections is blocked: found a non-https address in HTTPS proxy environment setting",
			cleanupFunc: func() {
				os.Unsetenv("HTTPS_PROXY")
			},
		},
		{
			name:      "https proxy set with https url and http conns blocked",
			allowHTTP: false,
			beforeFunc: func() {
				os.Setenv("HTTPS_PROXY", "https://example.com")
			},
			cleanupFunc: func() {
				os.Unsetenv("HTTPS_PROXY")
			},
		},
		{
			name:      "http allowed",
			allowHTTP: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cleanupFunc != nil {
				t.Cleanup(tt.cleanupFunc)
			}

			g := NewWithT(t)

			if tt.beforeFunc != nil {
				tt.beforeFunc()
			}

			opts := &controller.ConnectionOptions{
				AllowHTTP:            tt.allowHTTP,
				ProxyFromEnvironment: mockProxyEnvironmentConfig,
			}
			err := opts.CheckEnvironmentCompatibility()

			if tt.wantErrMsg != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(tt.wantErrMsg))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func mockProxyEnvironmentConfig() *httpproxy.Config {
	return &httpproxy.Config{
		HTTPProxy:  os.Getenv("HTTP_PROXY"),
		HTTPSProxy: os.Getenv("HTTPS_PROXY"),
	}
}
