package client

import (
	"github.com/spf13/pflag"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	flagQPS   = "kube-api-qps"
	flagBurst = "kube-api-burst"
)

// Options contains the configuration options for the Kubernetes client.
type Options struct {
	// QPS indicates the maximum queries-per-second of
	//requests sent to to the Kubernetes API, defaults to 50.
	QPS float32

	// Burst indicates the maximum burst queries-per-second of
	// requests sent to the Kubernetes API, defaults to 100.
	Burst int
}

// BindFlags will parse the given flagset for Kubernetes client option flags and
// set the Options accordingly.
func (o *Options) BindFlags(fs *pflag.FlagSet) {
	fs.Float32Var(&o.QPS, flagQPS, 50.0,
		"The maximum queries-per-second of requests sent to the Kubernetes API.")
	fs.IntVar(&o.Burst, flagBurst, 100,
		"The maximum burst queries-per-second of requests sent to the Kubernetes API.")
}

// GetConfigOrDie wraps ctrl.GetConfigOrDie and sets the QPS and Bust options
func GetConfigOrDie(opts Options) *rest.Config {
	restConfig := ctrl.GetConfigOrDie()
	restConfig.QPS = opts.QPS
	restConfig.Burst = opts.Burst
	return restConfig
}
