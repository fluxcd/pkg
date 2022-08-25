module github.com/fluxcd/pkg/git

go 1.18

require (
	// github.com/ProtonMail/go-crypto is a fork of golang.org/x/crypto
	// maintained by the ProtonMail team to continue to support the openpgp
	// module, after the Go team decided to no longer maintain it.
	// When in doubt (and not using openpgp), use /x/crypto.
	github.com/ProtonMail/go-crypto v0.0.0-20220824120805-4b6e5c587895
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/onsi/gomega v1.20.0
)

require (
	github.com/cloudflare/circl v1.1.0 // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/net v0.0.0-20220425223048-2871e0cb64e4 // indirect
	golang.org/x/sys v0.0.0-20220422013727-9388b58f7150 // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
