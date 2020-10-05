#!/bin/bash

set -eu

VERSION=${1:-3.3.4}

helm_url=https://get.helm.sh && \
curl -sL ${helm_url}/helm-v${VERSION}-linux-amd64.tar.gz | \
tar xz

mkdir -p $GITHUB_WORKSPACE/bin
cp ./linux-amd64/helm $GITHUB_WORKSPACE/bin
chmod +x $GITHUB_WORKSPACE/bin/helm

$GITHUB_WORKSPACE/bin/helm version

echo "$GITHUB_WORKSPACE/bin" >> $GITHUB_PATH
echo "$RUNNER_WORKSPACE/$(basename $GITHUB_REPOSITORY)/bin" >> $GITHUB_PATH
