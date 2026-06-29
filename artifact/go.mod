module github.com/fluxcd/pkg/artifact

go 1.26.0

replace (
	github.com/fluxcd/pkg/apis/meta => ../apis/meta
	github.com/fluxcd/pkg/lockedfile => ../lockedfile
	github.com/fluxcd/pkg/oci => ../oci
	github.com/fluxcd/pkg/sourceignore => ../sourceignore
	github.com/fluxcd/pkg/tar => ../tar
	github.com/fluxcd/pkg/version => ../version
)

require (
	github.com/cyphar/filepath-securejoin v0.6.1
	github.com/fluxcd/pkg/apis/meta v1.30.1
	github.com/fluxcd/pkg/lockedfile v0.8.0
	github.com/fluxcd/pkg/oci v0.68.0
	github.com/fluxcd/pkg/sourceignore v0.18.0
	github.com/fluxcd/pkg/tar v1.2.0
	github.com/onsi/gomega v1.40.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/go-digest/blake3 v0.0.0-20250813155314-89707e38ad1a
	github.com/spf13/pflag v1.0.10
	k8s.io/apimachinery v0.36.2
)

// Replace digest lib to master to gather access to BLAKE3.
// xref: https://github.com/opencontainers/go-digest/pull/66
replace github.com/opencontainers/go-digest => github.com/opencontainers/go-digest v1.0.1-0.20220411205349-bde1400a84be

require (
	github.com/Masterminds/semver/v3 v3.5.0 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.18.2 // indirect
	github.com/docker/cli v29.4.0+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.5 // indirect
	github.com/fluxcd/pkg/version v0.16.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-containerregistry v0.21.5 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/vbatts/tar-split v0.12.2 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/zeebo/blake3 v0.2.3 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260317180543-43fb72c5454a // indirect
	k8s.io/utils v0.0.0-20260210185600-b8788abfbbc2 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.2 // indirect
)
