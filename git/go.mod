module github.com/fluxcd/pkg/git

go 1.22.7

toolchain go1.23.0

replace github.com/fluxcd/pkg/auth => ../auth

require (
	github.com/ProtonMail/go-crypto v1.1.3
	github.com/cyphar/filepath-securejoin v0.3.4
	github.com/fluxcd/pkg/auth v0.0.1
	github.com/fluxcd/pkg/ssh v0.14.1
	github.com/onsi/gomega v1.34.2
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.16.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.8.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.10.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/bradleyfalzon/ghinstallation/v2 v2.12.0 // indirect
	github.com/cloudflare/circl v1.5.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/go-github/v66 v66.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	golang.org/x/crypto v0.30.0 // indirect
	golang.org/x/net v0.32.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
