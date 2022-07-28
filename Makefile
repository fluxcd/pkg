VER?=0.0.1
MODULES=$(shell find . -mindepth 2 -maxdepth 4 -type f -name 'go.mod' | cut -c 3- | sed 's|/[^/]*$$||' | sort -u | tr / :)
targets=$(addprefix test-, $(MODULES))
root_dir=$(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# Use $GOBIN from the enviornment if set, otherwise use ./bin
ifeq (,$(shell go env GOBIN))
GOBIN=$(root_dir)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Architecture to use envtest with
ENVTEST_ARCH ?= amd64
GO_TEST_ARGS ?= -race

# Repository root based on Git metadata
REPOSITORY_ROOT := $(shell git rev-parse --show-toplevel)

# Other dependency versions
ENVTEST_BIN_VERSION ?= 1.19.2

all:
	$(MAKE) $(targets)

tidy-%:
	@if [ "$(DIR)" = "git" ]; then \
		cd git && make tidy ;\
	else \
		cd $(subst :,/,$*) && go mod tidy -compat=1.17 ;\
	fi

fmt-%:
	@if [ "$(DIR)" = "git" ]; then \
		cd git && make fmt ;\
	else \
		cd $(subst :,/,$*) && go fmt ./... ;\
	fi

vet-%:
	@if [ "$(DIR)" = "git" ]; then \
		cd git && make vet ;\
	else \
		cd $(subst :,/,$*) && go vet ./... ;\
	fi

generate-%: controller-gen
# Run schemapatch to validate all the kubebuilder markers before generation
# Skip git/libgit2 as this isn't required for that package and increases
# the complexity unnecessarily
	@if [ "$(DIR)" = "git" ]; then \
		echo "skipping target 'generate' for git " ;\
	else \
		cd $(subst :,/,$*) ;\
		$(CONTROLLER_GEN) schemapatch:manifests="./" paths="./..." ;\
		$(CONTROLLER_GEN) object:headerFile="$(root_dir)/hack/boilerplate.go.txt" paths="./..." ;\
	fi

# Run tests
KUBEBUILDER_ASSETS?="$(shell $(ENVTEST) --arch=$(ENVTEST_ARCH) use -i $(ENVTEST_KUBERNETES_VERSION) --bin-dir=$(ENVTEST_ASSETS_DIR) -p path)"
DIR?=$*
test-%: tidy-% generate-% fmt-% vet-% install-envtest
	@if [ "$(DIR)" = "git" ]; then \
		cd git && make test ;\
	else \
		cd $(subst :,/,$*) && KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test $(GO_STATIC_FLAGS) ./... $(GO_TEST_ARGS) -coverprofile cover.out ;\
	fi

libgit2: $(LIBGIT2)  ## Detect or download libgit2 library

COSIGN = $(GOBIN)/cosign
$(LIBGIT2): $(MUSL-CC)
	$(call go-install-tool,$(COSIGN),github.com/sigstore/cosign/cmd/cosign@latest)

	cd git/libgit2 && IMG=$(LIBGIT2_IMG) TAG=$(LIBGIT2_TAG) PATH=$(PATH):$(GOBIN) ./hack/install-libraries.sh

$(MUSL-CC):
ifneq ($(shell uname -s),Darwin)
	cd git/libgit2 && ./hack/download-musl.sh
endif

release-%:
	$(eval REL_PATH=$(subst :,/,$*))
	@if ! test -f $(REL_PATH)/go.mod; then echo "Missing ./$(REL_PATH)/go.mod, terminating release process"; exit 1; fi
	git checkout main
	git pull
	git tag "$(REL_PATH)/v$(VER)"
	git push origin "$(REL_PATH)/v$(VER)"

# Find or download controller-gen
CONTROLLER_GEN = $(GOBIN)/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.2)

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
ENVTEST_KUBERNETES_VERSION?=latest
install-envtest: setup-envtest
	mkdir -p ${ENVTEST_ASSETS_DIR}
	$(ENVTEST) use $(ENVTEST_KUBERNETES_VERSION) --arch=$(ENVTEST_ARCH) --bin-dir=$(ENVTEST_ASSETS_DIR)

ENVTEST = $(GOBIN)/setup-envtest
.PHONY: envtest
setup-envtest: ## Download envtest-setup locally if necessary.
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

# go-install-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

fuzz-build:
	rm -rf $(shell pwd)/build/fuzz/
	mkdir -p $(shell pwd)/build/fuzz/out/

	docker build . --tag local-fuzzing:latest -f tests/fuzz/Dockerfile.builder
	docker run --rm \
		-e FUZZING_LANGUAGE=go -e SANITIZER=address \
		-e CIFUZZ_DEBUG='True' -e OSS_FUZZ_PROJECT_NAME=fluxcd \
		-v "$(shell pwd)/build/fuzz/out":/out \
		local-fuzzing:latest

fuzz-smoketest: fuzz-build
	docker run --rm \
		-v "$(shell pwd)/build/fuzz/out":/out \
		-v "$(shell pwd)/tests/fuzz/oss_fuzz_run.sh":/runner.sh \
		-e ENVTEST_BIN_VERSION=$(ENVTEST_KUBERNETES_VERSION) \
		local-fuzzing:latest \
		bash -c "/runner.sh"
