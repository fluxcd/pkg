module github.com/fluxcd/pkg/helmtestserver

go 1.16

replace github.com/fluxcd/pkg/testserver => ../testserver

require (
	github.com/fluxcd/pkg/testserver v0.1.0
	helm.sh/helm/v3 v3.6.1
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/yaml v1.2.0
)
