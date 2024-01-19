module github.com/fluxcd/pkg/git

go 1.20

require (
	// github.com/ProtonMail/go-crypto is a fork of golang.org/x/crypto
	// maintained by the ProtonMail team to continue to support the openpgp
	// module, after the Go team decided to no longer maintain it.
	// When in doubt (and not using openpgp), use /x/crypto.
	github.com/ProtonMail/go-crypto v0.0.0-20231012073058-a7379d079e0e
	github.com/cyphar/filepath-securejoin v0.2.4
	github.com/onsi/gomega v1.30.0
)

require (
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	golang.org/x/crypto v0.17.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
