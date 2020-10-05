#!/bin/bash

set -eu

VERSION=${1:-3.8.2}

kustomize_url=https://github.com/kubernetes-sigs/kustomize/releases/download && \
curl -sL ${kustomize_url}/kustomize%2Fv${VERSION}/kustomize_v${VERSION}_linux_amd64.tar.gz | \
tar xz

mkdir -p $GITHUB_WORKSPACE/bin
cp ./kustomize $GITHUB_WORKSPACE/bin
chmod +x $GITHUB_WORKSPACE/bin/kustomize

$GITHUB_WORKSPACE/bin/kustomize version

echo "$GITHUB_WORKSPACE/bin" >> $GITHUB_PATH
echo "$RUNNER_WORKSPACE/$(basename $GITHUB_REPOSITORY)/bin" >> $GITHUB_PATH
