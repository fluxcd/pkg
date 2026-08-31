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
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

func TestKubeConfig(t *testing.T) {
	tests := []struct {
		description        string
		in                 *rest.Config
		opts               KubeConfigOptions
		flowControlEnabled bool
		expected           *rest.Config
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
		{
			description:        "Flowcontrol enabled disables ratelimiting",
			in:                 &rest.Config{},
			opts:               KubeConfigOptions{},
			flowControlEnabled: true,
			expected: &rest.Config{
				QPS:     -1,
				Timeout: 30 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			flowControlChecker := func(context.Context, *rest.Config) (bool, error) { return tt.flowControlEnabled, nil }
			got := kubeconfig(t.Context(), tt.in, tt.opts, flowControlChecker)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func duration(d time.Duration) *time.Duration {
	return &d
}

const testKubeConfigInline = `apiVersion: v1
kind: Config
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://1.2.3.4
  name: development
contexts:
- context:
    cluster: development
    user: developer
  name: dev
current-context: dev
users:
- name: developer
  user:
    token: some-token`

func TestKubeConfigFromBytes(t *testing.T) {
	tests := []struct {
		name       string
		kubeConfig string
		wantErr    string
	}{
		{
			name:       "inline credentials",
			kubeConfig: testKubeConfigInline,
		},
		{
			name: "token file reference",
			kubeConfig: `apiVersion: v1
kind: Config
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://1.2.3.4
  name: development
contexts:
- context:
    cluster: development
    user: developer
  name: dev
current-context: dev
users:
- name: developer
  user:
    tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token`,
			wantErr: "user 'developer' references a tokenFile",
		},
		{
			name: "client certificate file reference",
			kubeConfig: `apiVersion: v1
kind: Config
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://1.2.3.4
  name: development
contexts:
- context:
    cluster: development
    user: developer
  name: dev
current-context: dev
users:
- name: developer
  user:
    client-certificate: /etc/ssl/client.crt
    client-key: /etc/ssl/client.key`,
			wantErr: "user 'developer' references a client-certificate file",
		},
		{
			name: "client key file reference",
			kubeConfig: `apiVersion: v1
kind: Config
clusters:
- cluster:
    insecure-skip-tls-verify: true
    server: https://1.2.3.4
  name: development
contexts:
- context:
    cluster: development
    user: developer
  name: dev
current-context: dev
users:
- name: developer
  user:
    client-key: /etc/ssl/client.key`,
			wantErr: "user 'developer' references a client-key file",
		},
		{
			name: "certificate authority file reference",
			kubeConfig: `apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: /etc/ssl/ca.crt
    server: https://1.2.3.4
  name: development
contexts:
- context:
    cluster: development
    user: developer
  name: dev
current-context: dev
users:
- name: developer
  user:
    token: some-token`,
			wantErr: "cluster 'development' references a certificate-authority file",
		},
		{
			name:       "invalid kubeconfig",
			kubeConfig: "bad",
			wantErr:    "couldn't get version/kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := KubeConfigFromBytes([]byte(tt.kubeConfig))
			if tt.wantErr != "" {
				assert.Nil(t, cfg)
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, "https://1.2.3.4", cfg.Host)
			assert.Equal(t, "some-token", cfg.BearerToken)
		})
	}
}

func TestValidateKubeConfig(t *testing.T) {
	assert.ErrorContains(t, ValidateKubeConfig(nil), "kubeconfig is nil")
	assert.ErrorContains(t, ValidateKubeConfig(&api.Config{
		Clusters: map[string]*api.Cluster{"c": nil},
	}), "cluster 'c' is nil")
	assert.ErrorContains(t, ValidateKubeConfig(&api.Config{
		AuthInfos: map[string]*api.AuthInfo{"u": nil},
	}), "user 'u' is nil")
	assert.NoError(t, ValidateKubeConfig(&api.Config{
		Clusters:  map[string]*api.Cluster{"c": {Server: "https://1.2.3.4", CertificateAuthorityData: []byte("ca")}},
		AuthInfos: map[string]*api.AuthInfo{"u": {Token: "t", ClientCertificateData: []byte("crt"), ClientKeyData: []byte("key")}},
	}))
}
