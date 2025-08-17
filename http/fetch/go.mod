module github.com/fluxcd/pkg/http/fetch

go 1.24.0

replace (
	github.com/fluxcd/pkg/tar => ../../tar
	github.com/fluxcd/pkg/testserver => ../../testserver
)

// Replace digest lib to master to gather access to BLAKE3.
// xref: https://github.com/opencontainers/go-digest/pull/66
replace github.com/opencontainers/go-digest => github.com/opencontainers/go-digest v1.0.1-0.20220411205349-bde1400a84be

require (
	github.com/fluxcd/pkg/tar v0.13.0
	github.com/fluxcd/pkg/testserver v0.12.0
	github.com/go-logr/logr v1.4.2
	github.com/hashicorp/go-retryablehttp v0.7.8
	github.com/onsi/gomega v1.37.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/go-digest/blake3 v0.0.0-20250116041648-1e56c6daea3b
)

require (
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/zeebo/blake3 v0.2.4 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
