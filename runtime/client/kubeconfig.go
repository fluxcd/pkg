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
	"time"

	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
)

const (
	flagInsecureKubeConfigExec = "insecure-kubeconfig-exec"
	flagInsecureKubeConfigTLS  = "insecure-kubeconfig-tls"
)

// KubeConfigOptions defines options for KubeConfig sanitization.
type KubeConfigOptions struct {
	// InsecureExecProvider enables the use of ExecProviders in kubeconfig.
	// To use this feature securely, it is recommended the use of restrictive
	// AppArmor and SELinux profiles to restrict what binaries can be executed.
	InsecureExecProvider bool

	// InsecureTLS disables TLS certificate verification. This is insecure and
	// should be used for testing purposes only.
	InsecureTLS bool

	// UserAgent defines a string to identify the caller.
	UserAgent string

	// Timeout defines the maximum length of time to wait before giving up on a server request.
	// A value of zero means no timeout.
	//
	// If not provided, it will be set to 30 seconds.
	Timeout *time.Duration
}

func (opts KubeConfigOptions) withDefaults() KubeConfigOptions {
	if opts.Timeout == nil {
		t := 30 * time.Second
		opts.Timeout = &t
	}
	return opts
}

// BindFlags will parse the given pflag.FlagSet for Kubernetes client option flags and set the Options accordingly.
func (o *KubeConfigOptions) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.InsecureExecProvider, flagInsecureKubeConfigExec, false,
		"Allow use of the user.exec section in kubeconfigs provided for remote apply.")
	fs.BoolVar(&o.InsecureTLS, flagInsecureKubeConfigTLS, false,
		"Allow that kubeconfigs provided for remote apply can disable TLS verification.")
}

// KubeConfig sanitises a kubeconfig represented as *rest.Config using
// KubeConfigOptions to inform the transformation decisions.
func KubeConfig(in *rest.Config, opts KubeConfigOptions) *rest.Config {
	var out *rest.Config

	if in != nil {
		opts = opts.withDefaults()
		out = &rest.Config{
			UserAgent: opts.UserAgent,
		}

		if opts.Timeout != nil {
			out.Timeout = *opts.Timeout
		}

		out.Host = in.Host
		out.APIPath = in.APIPath
		out.DisableCompression = in.DisableCompression

		out.TLSClientConfig = rest.TLSClientConfig{
			ServerName: in.TLSClientConfig.ServerName,
			CAData:     in.TLSClientConfig.CAData,
			CertData:   in.TLSClientConfig.CertData,
			KeyData:    in.TLSClientConfig.KeyData,
		}

		out.Username = in.Username
		out.Password = in.Password
		out.BearerToken = in.BearerToken

		if opts.InsecureTLS {
			out.TLSClientConfig.Insecure = in.TLSClientConfig.Insecure
		}

		if opts.InsecureExecProvider {
			out.ExecProvider = in.ExecProvider
		}
	}

	return out
}
