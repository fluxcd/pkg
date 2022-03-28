#!/usr/bin/env bash

# Copyright 2022 The Flux authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euxo pipefail

GOPATH="${GOPATH:-/root/go}"
GO_SRC="${GOPATH}/src"
PROJECT_PATH="github.com/fluxcd/pkg"

cd "${GO_SRC}"

# Move fuzzer to their respective directories.
# This removes dependency noises from the modules' go.mod and go.sum files.
cp "${PROJECT_PATH}/tests/fuzz/conditions_fuzzer.go" "${PROJECT_PATH}/runtime/conditions"
cp "${PROJECT_PATH}/tests/fuzz/events_fuzzer.go" "${PROJECT_PATH}/runtime/events"
cp "${PROJECT_PATH}/tests/fuzz/tls_fuzzer.go" "${PROJECT_PATH}/runtime/tls"
cp "${PROJECT_PATH}/tests/fuzz/untar_fuzzer.go" "${PROJECT_PATH}/untar"
cp "${PROJECT_PATH}/tests/fuzz/gitutil_fuzzer.go" "${PROJECT_PATH}/gitutil"


# compile fuzz tests for the runtime module
pushd "${PROJECT_PATH}/runtime"

go get -d github.com/AdaLogics/go-fuzz-headers
compile_go_fuzzer "${PROJECT_PATH}/runtime/conditions" FuzzGetterConditions fuzz_getter_conditions
compile_go_fuzzer "${PROJECT_PATH}/runtime/conditions" FuzzConditionsMatch fuzz_conditions_match
compile_go_fuzzer "${PROJECT_PATH}/runtime/conditions" FuzzPatchApply fuzz_patch_apply
compile_go_fuzzer "${PROJECT_PATH}/runtime/conditions" FuzzConditionsUnstructured fuzz_conditions_unstructured
compile_go_fuzzer "${PROJECT_PATH}/runtime/events" FuzzEventf fuzz_eventf
compile_go_fuzzer "${PROJECT_PATH}/runtime/tls" FuzzTlsConfig fuzz_tls_config

popd


# compile fuzz tests for the untar module
pushd "${PROJECT_PATH}/untar"

go get -d github.com/AdaLogics/go-fuzz-headers
compile_go_fuzzer "${PROJECT_PATH}/untar" FuzzUntar fuzz_untar

popd


# compile fuzz tests for the gitutil module
pushd "${PROJECT_PATH}/gitutil"

go get -d github.com/AdaLogics/go-fuzz-headers
compile_go_fuzzer "${PROJECT_PATH}/gitutil" FuzzLibGit2Error fuzz_libgit2_error

popd
