module github.com/fluxcd/pkg/git

go 1.17

replace (
	github.com/fluxcd/pkg/gittestserver => ../gittestserver
	github.com/fluxcd/pkg/gitutil => ../gitutil
	github.com/fluxcd/pkg/ssh => ../ssh
	github.com/fluxcd/pkg/version => ../version
)

require (
	github.com/Masterminds/semver/v3 v3.1.1
	// github.com/ProtonMail/go-crypto is a fork of golang.org/x/crypto
	// maintained by the ProtonMail team to continue to support the openpgp
	// module, after the Go team decided to no longer maintain it.
	// When in doubt (and not using openpgp), use /x/crypto.
	github.com/ProtonMail/go-crypto v0.0.0-20220517143526-88bb52951d5b
	github.com/fluxcd/gitkit v0.5.1
	github.com/fluxcd/pkg/gittestserver v0.5.4
	github.com/fluxcd/pkg/gitutil v0.1.0
	github.com/fluxcd/pkg/ssh v0.5.0
	github.com/fluxcd/pkg/version v0.1.0
	github.com/go-git/go-billy/v5 v5.3.1
	github.com/go-git/go-git/v5 v5.4.2
	github.com/onsi/gomega v1.19.0
	golang.org/x/crypto v0.0.0-20220525230936-793ad666bf5e
)

require (
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/xanzy/ssh-agent v0.3.1 // indirect
	golang.org/x/net v0.0.0-20220607020251-c690dde0001d // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)
