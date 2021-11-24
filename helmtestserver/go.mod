module github.com/fluxcd/pkg/helmtestserver

go 1.16

replace github.com/fluxcd/pkg/testserver => ../testserver

require (
	github.com/fluxcd/pkg/testserver v0.1.0
	helm.sh/helm/v3 v3.7.2
	sigs.k8s.io/yaml v1.3.0
)

replace (
	// Fix CVE-2021-41190
	github.com/containerd/containerd => github.com/containerd/containerd v1.5.8
)
