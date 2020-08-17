module github.com/fluxcd/pkg/testserver

go 1.14

// TODO(hidde): drop when PR is accepted:
//  https://github.com/sosedoff/gitkit/pull/21
replace github.com/sosedoff/gitkit => github.com/hiddeco/gitkit v0.2.1-0.20200422093229-4355fec70348

require (
	github.com/sosedoff/gitkit v0.2.0
	helm.sh/helm/v3 v3.3.0
	rsc.io/letsencrypt v0.0.3 // indirect
	sigs.k8s.io/yaml v1.2.0
)
