#!/usr/bin/env bash

# This script runs e2e tests for pkg/git/gogit.

set -o errexit

PROJECT_DIR=$(git rev-parse --show-toplevel)
DIR="$(cd "$(dirname "$0")" && pwd)"
GITLAB_CONTAINER=gitlab-flux-e2e

if [[ "${GO_TEST_PREFIX}" = "" ]] || [[ "${GO_TEST_PREFIX}" = *"TestGitLabCEE2E"* ]]; then
    # Cleanup gitlab container if persistence is not enabled.
    if [[ -z "${PERSIST_GITLAB}" ]]; then
        trap "docker kill ${GITLAB_CONTAINER} && docker rm ${GITLAB_CONTAINER}" EXIT
    fi
    source "${DIR}/setup_gitlab.sh"
fi

cd "${DIR}"
CGO_ENABLED=1 go test -v -tags 'netgo,osusergo,static_build,e2e' -race -run "^${GO_TEST_PREFIX}.*" ./...
