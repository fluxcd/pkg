module github.com/fluxcd/pkg/ssh

go 1.17

// Fix for CVE-2020-29652: https://github.com/golang/crypto/commit/8b5274cf687fd9316b4108863654cc57385531e8
// Fix for CVE-2021-43565: https://github.com/golang/crypto/commit/5770296d904e90f15f38f77dfc2e43fdf5efc083
require golang.org/x/crypto v0.0.0-20220315160706-3147a52a75dd

require golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1 // indirect
