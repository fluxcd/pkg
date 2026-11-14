module github.com/fluxcd/pkg/testserver

go 1.27.0

require (
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/go-digest/blake3 v0.0.0-20261015205711-63db54280247
)

require (
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/zeebo/blake3 v0.2.3 // indirect
	golang.org/x/sys v0.13.0 // indirect
)

// Replace digest lib to master to gather access to BLAKE3.
// xref: https://github.com/opencontainers/go-digest/pull/66
replace github.com/opencontainers/go-digest => github.com/opencontainers/go-digest v1.0.1-0.20220411205349-bde1400a84be
