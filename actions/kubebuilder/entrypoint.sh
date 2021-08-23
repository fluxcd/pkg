#!/bin/bash

set -eu

VERSION=${1:-2.3.1}

curl -sL https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${VERSION}/kubebuilder_${VERSION}_linux_amd64.tar.gz | tar -xz -C /tmp/

mkdir -p $GITHUB_WORKSPACE/kubebuilder
mv /tmp/kubebuilder_${VERSION}_linux_amd64/* $GITHUB_WORKSPACE/kubebuilder/
ls -lh $GITHUB_WORKSPACE/kubebuilder/bin

echo "$GITHUB_WORKSPACE/kubebuilder/bin" >> $GITHUB_PATH
echo "$RUNNER_WORKSPACE/$(basename $GITHUB_REPOSITORY)/kubebuilder/bin" >> $GITHUB_PATH
