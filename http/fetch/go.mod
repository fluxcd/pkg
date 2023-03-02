module github.com/fluxcd/pkg/http/fetch

go 1.18

replace (
	github.com/fluxcd/pkg/tar => ../../tar
	github.com/fluxcd/pkg/testserver => ../../testserver
)

require (
	github.com/fluxcd/pkg/tar v0.2.0
	github.com/fluxcd/pkg/testserver v0.4.0
	github.com/hashicorp/go-retryablehttp v0.7.2
	github.com/onsi/gomega v1.27.2
)

require (
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
