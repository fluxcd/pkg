module github.com/fluxcd/pkg/helmtestserver

go 1.16

replace github.com/fluxcd/pkg/testserver => ../testserver

require (
	github.com/fluxcd/pkg/testserver v0.1.0
	github.com/garyburd/redigo v1.6.3 // indirect
	helm.sh/helm/v3 v3.7.1
	sigs.k8s.io/yaml v1.3.0
)

replace (
	// Fix CVE-2021-41190
	github.com/containerd/containerd => github.com/containerd/containerd v1.5.8

	// Fix CVE-2021-41190
	github.com/opencontainers/image-spec => github.com/opencontainers/image-spec v1.0.2
)
