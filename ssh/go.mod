module github.com/fluxcd/pkg/ssh

go 1.18

// Fix for CVE-2020-29652: https://github.com/golang/crypto/commit/8b5274cf687fd9316b4108863654cc57385531e8
// Fix for CVE-2021-43565: https://github.com/golang/crypto/commit/5770296d904e90f15f38f77dfc2e43fdf5efc083
require golang.org/x/crypto v0.0.0-20220427172511-eb4f295cb31f

require github.com/onsi/gomega v1.19.0

require (
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
