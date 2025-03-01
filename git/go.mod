module github.com/fluxcd/pkg/git

go 1.23.0

replace (
	github.com/fluxcd/pkg/auth => ../auth
	github.com/fluxcd/pkg/ssh => ../ssh
)

require (
	github.com/ProtonMail/go-crypto v1.1.5
	github.com/cyphar/filepath-securejoin v0.4.1
	github.com/fluxcd/pkg/auth v0.2.0
	github.com/fluxcd/pkg/ssh v0.17.0
	github.com/onsi/gomega v1.36.2
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.17.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.8.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.10.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.4.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bradleyfalzon/ghinstallation/v2 v2.14.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.5.0 // indirect
	github.com/fluxcd/pkg/cache v0.4.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.1 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-github/v69 v69.2.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	golang.org/x/crypto v0.34.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/protobuf v1.36.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
