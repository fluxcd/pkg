# Release Documentation

## Releasing minor versions (development branch, `main`)

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

## Releasing patch versions (release branches, `flux/v2.<minor>.x`)

This process is intended to be run locally, in the clone of a Flux maintainer,
properly configured with commit signing and GitHub credentials.

First, a preparation PR must be created bumping all the Go modules
from this repository that have changed since their latest version
in the release branch.

Run the following commands:

1. `git checkout flux/v2.<minor>.x`
2. `git pull`
3. `make prep`

If there are any changes, commit, open a PR `Prepare for release` against
the release branch and merge.

If no changes are needed, then:

1. `git checkout flux/v2.<minor>.x`
2. `git pull`
3. `make release`

Both `make` commands will show a plan of the changes they
will make and ask for confirmation.

## Backporting fixes to release branches

When backporting fixes to release branches through the `backport:<branch>` label,
you can checkout the backport PR branch, i.e. `backport-<PR number>-to-<branch>`,
and run `make prep` to update the Go modules in the backport branch. Then commit
`Prepare for release` and push the branch to update the backport PR. Once it's
merged, the release branch will be ready for `make release`.

## New test Go modules

Whenever adding new test Go modules like `git/internal/e2e` or `tests/integration`,
you must also add the module path to the `testModules` slice in `cmd/internal/test_modules.go`.
This is necessary to ensure that these modules are not considered for tagging when running
the `make release` command.
