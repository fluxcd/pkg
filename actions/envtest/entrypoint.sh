#!/bin/bash

set -eu

# Unset any $GOROOT passed in from the host.
unset GOROOT

# envtest kubernetes version.
VERSION=${1:-latest}

go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
setup-envtest use $VERSION

ENVTEST_DIR=$(setup-envtest use -i $VERSION -p path)
SETUP_ENVTEST_PATH=$(which setup-envtest)
ENVTEST_WORKSPACE_PATH=$RUNNER_WORKSPACE/$(basename $GITHUB_REPOSITORY)/envtest

mkdir -p $GITHUB_WORKSPACE/envtest
mv $ENVTEST_DIR/* $GITHUB_WORKSPACE/envtest
mv $SETUP_ENVTEST_PATH $GITHUB_WORKSPACE/envtest
ls -lh $GITHUB_WORKSPACE/envtest

echo "$GITHUB_WORKSPACE/envtest" >> $GITHUB_PATH
echo "$ENVTEST_WORKSPACE_PATH" >> $GITHUB_PATH
echo "KUBEBUILDER_ASSETS=$ENVTEST_WORKSPACE_PATH" >> $GITHUB_ENV

# Cleanup cache to avoid affecting other builds. Some go builds may get
# affected by this cache populated in the above operations.
rm -rf /github/home/.cache
