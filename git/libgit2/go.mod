module github.com/fluxcd/pkg/git/libgit2

go 1.19

replace (
	github.com/fluxcd/pkg/git => ../../git
	// Enables the use of pkg/git/gogit/fs.
	github.com/fluxcd/pkg/git/gogit => ../gogit
	github.com/fluxcd/pkg/gittestserver => ../../gittestserver
	github.com/fluxcd/pkg/http/transport => ../../http/transport
	github.com/fluxcd/pkg/ssh => ../../ssh
	github.com/fluxcd/pkg/version => ../../version
)

// Fix CVE-2022-1996 (for v2, Go Modules incompatible)
replace github.com/emicklei/go-restful => github.com/emicklei/go-restful v2.16.0+incompatible

// Flux has its own git2go fork to enable changes in behaviour for improved
// reliability.
//
// For more information refer to:
// - fluxcd/image-automation-controller/#339.
// - libgit2/git2go#918.
replace github.com/libgit2/git2go/v34 => github.com/fluxcd/git2go/v34 v34.0.0

require (
	github.com/Masterminds/semver/v3 v3.2.0
	github.com/ProtonMail/go-crypto v0.0.0-20230214155104-81033d7f4442
	github.com/armon/go-socks5 v0.0.0-20160902184237-e75332964ef5
	github.com/elazarl/goproxy v0.0.0-20221015165544-a0805db90819
	github.com/fluxcd/gitkit v0.6.0
	github.com/fluxcd/pkg/git v0.10.0
	github.com/fluxcd/pkg/git/gogit v0.7.0
	github.com/fluxcd/pkg/gittestserver v0.8.1
	github.com/fluxcd/pkg/http/transport v0.1.0
	github.com/fluxcd/pkg/ssh v0.7.1
	github.com/fluxcd/pkg/version v0.2.1
	github.com/go-git/go-billy/v5 v5.4.1
	github.com/go-logr/logr v1.2.3
	github.com/google/uuid v1.3.0
	github.com/libgit2/git2go/v34 v34.0.0
	github.com/onsi/gomega v1.26.0
	golang.org/x/crypto v0.6.0
	golang.org/x/net v0.7.0
	sigs.k8s.io/controller-runtime v0.14.4
)

require (
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cloudflare/circl v1.3.2 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.1 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fluxcd/go-git/v5 v5.0.0-20221219190809-2e5c9d01cfc4 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pjbgf/sha1cd v0.2.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	github.com/skeema/knownhosts v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	golang.org/x/mod v0.8.0 // indirect
	golang.org/x/oauth2 v0.5.0 // indirect
	golang.org/x/sys v0.5.0 // indirect
	golang.org/x/term v0.5.0 // indirect
	golang.org/x/text v0.7.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.6.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.26.1 // indirect
	k8s.io/apiextensions-apiserver v0.26.1 // indirect
	k8s.io/apimachinery v0.26.1 // indirect
	k8s.io/client-go v0.26.1 // indirect
	k8s.io/component-base v0.26.1 // indirect
	k8s.io/klog/v2 v2.90.0 // indirect
	k8s.io/kube-openapi v0.0.0-20230202010329-39b3636cbaa3 // indirect
	k8s.io/utils v0.0.0-20230209194617-a36077c30491 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)
