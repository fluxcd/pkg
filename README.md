# pkg

[![godev](https://img.shields.io/static/v1?label=godev&message=reference&color=00add8)](https://pkg.go.dev/github.com/fluxcd/pkg)
[![build](https://github.com/fluxcd/pkg/workflows/build/badge.svg)](https://github.com/fluxcd/pkg/actions)

GitOps Toolkit common packages.

## New test Go modules

Whenever adding new test Go modules like `git/internal/e2e` or `oci/tests/integration`,
you must also add the module path to the `testModules` slice in `cmd/internal/test_modules.go`.
This is necessary to ensure that these modules are not considered for tagging when running
the `make release` command.

## Release procedure

This process is intended to be run locally, in the clone of a Flux maintainer,
properly configured with commit signing and GitHub credentials.

First, a preparation PR must be created bumping all the Go modules
from this repository that have changed since their latest version.

Run the following commands:

1. `git checkout main`
2. `git pull`
3. `make prep`

If there are any changes, commit, open a PR `Prepare for release` and merge.

If no changes are needed, then:

1. `git checkout main`
2. `git pull`
3. `make release`

Both `make` commands will show a plan of the changes they
will make and ask for confirmation.
