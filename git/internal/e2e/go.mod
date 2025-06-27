module github.com/fluxcd/pkg/git/internal/e2e

go 1.24.0

replace (
	github.com/fluxcd/pkg/cache => ../../../cache
	github.com/fluxcd/pkg/git => ../../../git
	github.com/fluxcd/pkg/git/gogit => ../../gogit
	github.com/fluxcd/pkg/gittestserver => ../../../gittestserver
	github.com/fluxcd/pkg/ssh => ../../../ssh
	github.com/fluxcd/pkg/version => ../../../version
)

require (
	github.com/fluxcd/go-git-providers v0.22.0
	github.com/fluxcd/pkg/git v0.32.0
	github.com/fluxcd/pkg/git/gogit v0.35.1
	github.com/fluxcd/pkg/gittestserver v0.17.0
	github.com/fluxcd/pkg/ssh v0.19.0
	github.com/go-git/go-git/v5 v5.16.2
	github.com/go-logr/logr v1.4.2
	github.com/google/go-github/v66 v66.0.0
	github.com/google/uuid v1.6.0
	github.com/onsi/gomega v1.37.0
)

require (
	dario.cat/mergo v1.0.1 // indirect
	github.com/Masterminds/semver/v3 v3.3.1 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/ProtonMail/go-crypto v1.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bradleyfalzon/ghinstallation/v2 v2.15.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudflare/circl v1.6.1 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fluxcd/gitkit v0.6.0 // indirect
	github.com/fluxcd/pkg/cache v0.9.0 // indirect
	github.com/fluxcd/pkg/version v0.7.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-git/go-billy/v5 v5.6.2 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-github/v71 v71.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pjbgf/sha1cd v0.3.2 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.63.0 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/sergi/go-diff v1.3.2-0.20230802210424-5b0b94c5c0d3 // indirect
	github.com/skeema/knownhosts v1.3.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/xanzy/go-gitlab v0.114.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.39.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/oauth2 v0.28.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
