module github.com/fluxcd/pkg/ssa

go 1.16

require (
	github.com/google/go-cmp v0.5.6
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.22.2
	// pin cli-utils to avoid kustomize@v2.0.3+incompatible dependency in v0.25.0
	sigs.k8s.io/cli-utils v0.25.1-0.20210608181808-f3974341173a
	sigs.k8s.io/controller-runtime v0.10.1
	sigs.k8s.io/yaml v1.3.0
)
