module github.com/fluxcd/pkg/runtime

go 1.16

replace (
	github.com/fluxcd/pkg/apis/acl => ../apis/acl
	github.com/fluxcd/pkg/apis/meta => ../apis/meta
)

require (
	github.com/fluxcd/pkg/apis/acl v0.0.1
	github.com/fluxcd/pkg/apis/meta v0.11.0-rc.1
	github.com/go-logr/logr v0.4.0
	github.com/google/go-cmp v0.5.5
	github.com/hashicorp/go-retryablehttp v0.6.8
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.11.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	go.uber.org/zap v1.17.0
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e // indirect
	golang.org/x/sys v0.0.0-20210616094352-59db8d763f22 // indirect
	golang.org/x/tools v0.1.4 // indirect
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/klog/v2 v2.8.0
	sigs.k8s.io/controller-runtime v0.9.2
)
