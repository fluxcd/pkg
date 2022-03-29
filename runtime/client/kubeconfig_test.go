/*
Copyright 2022 The Flux authors

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
	"reflect"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestKubeConfig(t *testing.T) {
	tests := []struct {
		description string
		in          *rest.Config
		opts        KubeConfigOptions
		expected    *rest.Config
	}{
		{
			description: "ignore nil configs",
			in:          nil,
			expected:    nil,
		},
		{
			description: "ignore ExecProvider by default",
			in: &rest.Config{
				ExecProvider: &api.ExecConfig{
					Command: "any-command",
				},
			},
			opts:     KubeConfigOptions{Timeout: duration(0)},
			expected: &rest.Config{},
		},
		{
			description: "copy ExecProvider settings when enabled on kubeconfigOptions",
			in: &rest.Config{
				ExecProvider: &api.ExecConfig{
					Command: "any-command",
				},
			},
			opts: KubeConfigOptions{
				InsecureExecProvider: true,
				Timeout:              duration(0),
			},
			expected: &rest.Config{
				ExecProvider: &api.ExecConfig{
					Command: "any-command",
				},
			},
		},
		{
			description: "ignore TLSClientConfig.Insecure by default",
			in: &rest.Config{
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			},
			opts:     KubeConfigOptions{Timeout: duration(0)},
			expected: &rest.Config{},
		},
		{
			description: "copy TLSClientConfig.Insecure value when enabled on kubeconfigOptions",
			in: &rest.Config{
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			},
			opts: KubeConfigOptions{
				InsecureTLS: true,
				Timeout:     duration(0),
			},
			expected: &rest.Config{
				TLSClientConfig: rest.TLSClientConfig{
					Insecure: true,
				},
			},
		},
		{
			description: "core values should not be changed",
			in: &rest.Config{
				Host:               "host",
				APIPath:            "api-path",
				DisableCompression: true,
				Username:           "username",
				Password:           "password",
				BearerToken:        "beartoken",

				TLSClientConfig: rest.TLSClientConfig{
					ServerName: "server-name",
					CAData:     []byte("CA-data"),
					CertData:   []byte("cert-data"),
					KeyData:    []byte("key-data"),
				},
			},
			opts: KubeConfigOptions{Timeout: duration(0)},
			expected: &rest.Config{
				Host:               "host",
				APIPath:            "api-path",
				DisableCompression: true,
				Username:           "username",
				Password:           "password",
				BearerToken:        "beartoken",

				TLSClientConfig: rest.TLSClientConfig{
					ServerName: "server-name",
					CAData:     []byte("CA-data"),
					CertData:   []byte("cert-data"),
					KeyData:    []byte("key-data"),
				},
			},
		},
		{
			description: "useragent and timeout cannot be overwriten",
			in: &rest.Config{
				UserAgent: "Kubernetes Kubernetes Kubernetes",
				Timeout:   9999 * time.Second,
			},
			opts: KubeConfigOptions{
				UserAgent: "Flux Remote Apply",
				Timeout:   duration(30 * time.Second),
			},
			expected: &rest.Config{
				UserAgent: "Flux Remote Apply",
				Timeout:   30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		got := KubeConfig(tt.in, tt.opts)
		if !reflect.DeepEqual(tt.expected, got) {
			t.Error(tt.description)
		}
	}
}

func duration(d time.Duration) *time.Duration {
	return &d
}
