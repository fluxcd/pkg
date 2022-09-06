
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
TMP_DIR=$(mktemp -d /tmp/oss_fuzz-XXXXXX)

cleanup(){
	rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

install_deps(){
	if ! command -v go-118-fuzz-build &> /dev/null || ! command -v addimport &> /dev/null; then
		mkdir -p "${TMP_DIR}/go-118-fuzz-build"

		git clone https://github.com/AdamKorcz/go-118-fuzz-build "${TMP_DIR}/go-118-fuzz-build"
		cd "${TMP_DIR}/go-118-fuzz-build"
		go build -o "${GOPATH}/bin/go-118-fuzz-build"

		cd addimport
		go build -o "${GOPATH}/bin/addimport"
	fi

	if ! command -v goimports &> /dev/null; then
		go install golang.org/x/tools/cmd/goimports@latest
	fi
}

# Removes the content of test funcs which could cause the Fuzz
# tests to break.
remove_test_funcs(){
	filename=$1

	echo "removing co-located *testing.T"
	sed -i -e '/func Test.*testing.T) {$/ {:r;/\n}/!{N;br}; s/\n.*\n/\n/}' "${filename}"
	# Remove gomega reference as it is not used by Fuzz tests.
	sed -i 's;. "github.com/onsi/gomega";;g' "${filename}"

	# After removing the body of the go testing funcs, consolidate the imports.
	goimports -w "${filename}"
}

install_deps

cd "${GO_SRC}/${PROJECT_PATH}"
modules=$(find . -mindepth 2 -maxdepth 4 -type f -name 'go.mod' | cut -c 3- | sed 's|/[^/]*$$||' | sort -u | sed 's;/go.mod;;g')

for module in ${modules}; do

	cd "${GO_SRC}/${PROJECT_PATH}/${module}"

	test_files=$(grep -r --include='**_test.go' --files-with-matches 'func Fuzz' . || echo "")
	if [ -z "${test_files}" ]; then
		continue
	fi

	go get github.com/AdamKorcz/go-118-fuzz-build/utils

	# Iterate through all Go Fuzz targets, compiling each into a fuzzer.
	for file in ${test_files}; do
		remove_test_funcs "${file}"

		targets=$(grep -oP 'func \K(Fuzz\w*)' "${file}")
		for target_name in ${targets}; do
			# Transform module path into module name (e.g. git/libgit2 to git_libgit2).
			module_name=$(echo ${module} | tr / _)
			# Compose fuzzer name based on the lowercase version of the func names.
			# The module name is added after the fuzz prefix, for better discoverability.
			fuzzer_name=$(echo "${target_name}" | tr '[:upper:]' '[:lower:]' | sed "s;fuzz_;fuzz_${module_name}_;g")
			target_dir=$(dirname "${file}")

			echo "Building ${file}.${target_name} into ${fuzzer_name}"
			compile_native_go_fuzzer "${target_dir}" "${target_name}" "${fuzzer_name}"
		done
	done

done
