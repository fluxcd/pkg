module github.com/fluxcd/pkg/http/fetch

go 1.20

replace (
	github.com/fluxcd/pkg/tar => ../../tar
	github.com/fluxcd/pkg/testserver => ../../testserver
)

// Replace digest lib to master to gather access to BLAKE3.
// xref: https://github.com/opencontainers/go-digest/pull/66
replace github.com/opencontainers/go-digest => github.com/opencontainers/go-digest v1.0.1-0.20220411205349-bde1400a84be

require (
	github.com/fluxcd/pkg/tar v0.2.0
	github.com/fluxcd/pkg/testserver v0.4.0
	github.com/hashicorp/go-retryablehttp v0.7.4
	github.com/onsi/gomega v1.27.7
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/go-digest/blake3 v0.0.0-20230329235805-65fac7b55eb7
)

require (
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/zeebo/blake3 v0.1.1 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
