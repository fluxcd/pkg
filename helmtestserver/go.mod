module github.com/fluxcd/pkg/helmtestserver

go 1.14

replace github.com/fluxcd/pkg/testserver => ../testserver

require (
	github.com/fluxcd/pkg/testserver v0.0.0-00010101000000-000000000000
	helm.sh/helm/v3 v3.3.0
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/yaml v1.2.0
)
