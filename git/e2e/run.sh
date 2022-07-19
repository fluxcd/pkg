#!/bin/bash
# This script runs e2e tests for pkg/git.

DIR="$(cd "$(dirname "$0")" && pwd)"
source "$DIR"/setup_gitlab.sh
go test -v -tags e2e ./...
# cleanup
docker kill gitlab && docker rm gitlab
