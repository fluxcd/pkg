module github.com/fluxcd/pkg/runtime

go 1.14

replace github.com/fluxcd/pkg/apis/meta => ../apis/meta

require (
	github.com/fluxcd/pkg/apis/meta v0.0.2
	github.com/go-logr/logr v0.1.0
	github.com/hashicorp/go-retryablehttp v0.6.7
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.10.0
	k8s.io/api v0.18.9
	k8s.io/apimachinery v0.18.9
	sigs.k8s.io/controller-runtime v0.6.3
)
