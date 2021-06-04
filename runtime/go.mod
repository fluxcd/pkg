module github.com/fluxcd/pkg/runtime

go 1.15

replace github.com/fluxcd/pkg/apis/meta => ../apis/meta

require (
	github.com/fluxcd/pkg/apis/meta v0.9.0
	github.com/go-logr/logr v0.3.0
	github.com/hashicorp/go-retryablehttp v0.6.8
	github.com/prometheus/client_golang v1.7.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	go.uber.org/zap v1.16.0
	k8s.io/api v0.20.7
	k8s.io/apimachinery v0.20.7
	k8s.io/client-go v0.20.7
	sigs.k8s.io/controller-runtime v0.8.3
)
