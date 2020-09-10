#!/bin/bash

set -eu

VERSION=${1:-1.19.1}

curl -sL https://storage.googleapis.com/kubernetes-release/release/v${VERSION}/bin/linux/amd64/kubectl > kubectl

mkdir -p $GITHUB_WORKSPACE/bin
cp ./kubectl $GITHUB_WORKSPACE/bin
chmod +x $GITHUB_WORKSPACE/bin/kubectl

$GITHUB_WORKSPACE/bin/kubectl version --client

echo "::add-path::$GITHUB_WORKSPACE/bin"
echo "::add-path::$RUNNER_WORKSPACE/$(basename $GITHUB_REPOSITORY)/bin"
