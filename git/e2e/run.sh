#!/bin/bash
# This script runs e2e tests for pkg/git.

set -o errexit
DIR="$(cd "$(dirname "$0")" && pwd)"

source "$DIR"/setup_gitlab.sh

cd "$DIR"
PKG_CONFIG_PATH=$PKG_CONFIG_PATH CGO_LDFLAGS=$CGO_LDFLAGS go test -v -tags 'netgo,osusergo,static_build,e2e' -race ./...

# cleanup
docker kill gitlab && docker rm gitlab

