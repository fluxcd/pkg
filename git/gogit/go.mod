module github.com/fluxcd/pkg/git/gogit

go 1.18

replace (
	github.com/fluxcd/pkg/git => ../../git
	github.com/fluxcd/pkg/gittestserver => ../../gittestserver
	github.com/fluxcd/pkg/gitutil => ../../gitutil
	github.com/fluxcd/pkg/ssh => ../../ssh
	github.com/fluxcd/pkg/version => ../../version
)

require (
	github.com/Masterminds/semver/v3 v3.1.1
	github.com/fluxcd/gitkit v0.6.0
	github.com/fluxcd/go-git/v5 v5.0.0-20221104190732-329fd6659b10
	github.com/fluxcd/pkg/git v0.6.1
	github.com/fluxcd/pkg/gittestserver v0.7.0
	github.com/fluxcd/pkg/gitutil v0.2.0
	github.com/fluxcd/pkg/ssh v0.6.0
	github.com/fluxcd/pkg/version v0.2.0
	github.com/go-git/go-billy/v5 v5.3.1
	github.com/onsi/gomega v1.22.1
	golang.org/x/crypto v0.1.0
	golang.org/x/sys v0.1.0
)

require (
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20221026131551-cf6655e29de4 // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/cloudflare/circl v1.1.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/skeema/knownhosts v1.1.0 // indirect
	github.com/xanzy/ssh-agent v0.3.2 // indirect
	golang.org/x/net v0.1.0 // indirect
	golang.org/x/text v0.4.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
