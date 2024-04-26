module github.com/fluxcd/pkg/http/fetch

go 1.22.0

replace (
	github.com/fluxcd/pkg/tar => ../../tar
	github.com/fluxcd/pkg/testserver => ../../testserver
)

// Replace digest lib to master to gather access to BLAKE3.
// xref: https://github.com/opencontainers/go-digest/pull/66
replace github.com/opencontainers/go-digest => github.com/opencontainers/go-digest v1.0.1-0.20220411205349-bde1400a84be

require (
	github.com/fluxcd/pkg/tar v0.7.0
	github.com/fluxcd/pkg/testserver v0.7.0
	github.com/go-logr/logr v1.4.1
	github.com/hashicorp/go-retryablehttp v0.7.5
	github.com/onsi/gomega v1.32.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/go-digest/blake3 v0.0.0-20231025023718-d50d2fec9c98
)

require (
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.6 // indirect
	github.com/zeebo/blake3 v0.2.3 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
