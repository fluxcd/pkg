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
	"fmt"
	"time"

	"github.com/fluxcd/cli-utils/pkg/flowcontrol"
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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

// KubeConfigFromBytes builds a *rest.Config from the given serialized
// kubeconfig. The kubeconfig must be self-contained: credentials and
// certificates have to be embedded inline (`token`, `client-certificate-data`,
// `client-key-data`, `certificate-authority-data`). References to files on the
// local filesystem (`tokenFile`, `client-certificate`, `client-key`,
// `certificate-authority`) are rejected.
//
// The returned config is not sanitized; callers are expected to pass it
// through KubeConfig.
func KubeConfigFromBytes(kubeConfig []byte) (*rest.Config, error) {
	cfg, err := clientcmd.Load(kubeConfig)
	if err != nil {
		return nil, err
	}
	if err := ValidateKubeConfig(cfg); err != nil {
		return nil, err
	}
	return clientcmd.NewDefaultClientConfig(*cfg, &clientcmd.ConfigOverrides{}).ClientConfig()
}

// ValidateKubeConfig returns an error if any cluster or user entry in the
// given kubeconfig references a file on the local filesystem instead of
// embedding the data inline.
func ValidateKubeConfig(cfg *clientcmdapi.Config) error {
	if cfg == nil {
		return fmt.Errorf("kubeconfig is nil")
	}
	for name, cluster := range cfg.Clusters {
		if cluster == nil {
			return fmt.Errorf("cluster '%s' is nil", name)
		}
		if cluster.CertificateAuthority != "" {
			return fmt.Errorf("cluster '%s' references a certificate-authority file, only certificate-authority-data is supported", name)
		}
	}
	for name, user := range cfg.AuthInfos {
		if user == nil {
			return fmt.Errorf("user '%s' is nil", name)
		}
		switch {
		case user.TokenFile != "":
			return fmt.Errorf("user '%s' references a tokenFile, only an inline token is supported", name)
		case user.ClientCertificate != "":
			return fmt.Errorf("user '%s' references a client-certificate file, only client-certificate-data is supported", name)
		case user.ClientKey != "":
			return fmt.Errorf("user '%s' references a client-key file, only client-key-data is supported", name)
		}
	}
	return nil
}

// KubeConfig sanitises a kubeconfig represented as *rest.Config using
// KubeConfigOptions to inform the transformation decisions.
func KubeConfig(ctx context.Context, in *rest.Config, opts KubeConfigOptions) *rest.Config {
	return kubeconfig(ctx, in, opts, flowcontrol.IsEnabled)
}

func kubeconfig(ctx context.Context, in *rest.Config, opts KubeConfigOptions, flowcontrolChecker func(context.Context, *rest.Config) (bool, error)) *rest.Config {
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

		out.WrapTransport = in.WrapTransport

		out.Proxy = in.Proxy

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

	enabled, err := flowcontrolChecker(ctx, out)
	if err == nil && enabled {
		// A negative QPS indicates that the client should not have a rate limiter.
		// Ref: https://github.com/kubernetes/kubernetes/blob/v1.24.0/staging/src/k8s.io/client-go/rest/config.go#L354-L364
		out.QPS = -1
	}

	return out
}
