module github.com/fluxcd/pkg/spiffe

go 1.27.0

require (
	github.com/fluxcd/pkg/apis/crypto v0.5.0
	github.com/go-jose/go-jose/v4 v4.1.6
	github.com/gobuffalo/flect v1.0.3
	github.com/onsi/gomega v1.44.0
	github.com/spiffe/go-spiffe/v2 v2.9.0
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-containerregistry v0.22.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260819154853-08b0e4226688 // indirect
)

replace github.com/fluxcd/pkg/apis/crypto => ../apis/crypto
