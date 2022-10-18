module github.com/fluxcd/pkg/http/fetch

go 1.18

replace (
	github.com/fluxcd/pkg/tar => ../../tar
	github.com/fluxcd/pkg/testserver => ../../testserver
)

require (
	github.com/fluxcd/pkg/tar v0.2.0
	github.com/fluxcd/pkg/testserver v0.3.0
	github.com/hashicorp/go-retryablehttp v0.7.1
	github.com/onsi/gomega v1.21.1
)

require (
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.1 // indirect
	golang.org/x/net v0.0.0-20220722155237-a158d28d115b // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
