module github.com/fluxcd/pkg/git

go 1.19

require (
	// github.com/ProtonMail/go-crypto is a fork of golang.org/x/crypto
	// maintained by the ProtonMail team to continue to support the openpgp
	// module, after the Go team decided to no longer maintain it.
	// When in doubt (and not using openpgp), use /x/crypto.
	github.com/ProtonMail/go-crypto v0.0.0-20230201104953-d1d05f4e2bfb
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/onsi/gomega v1.24.1
)

require (
	github.com/cloudflare/circl v1.3.2 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	golang.org/x/crypto v0.6.0 // indirect
	golang.org/x/net v0.7.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
