VER?=0.0.1
MODULES=$(shell find . -mindepth 2 -maxdepth 4 -type f -name 'go.mod' | cut -c 3- | sed 's|/[^/]*$$||' | sort -u | tr / :)
root_dir=$(shell git rev-parse --show-toplevel)

# Use $GOBIN from the environment if set, otherwise use ./bin
ifeq (,$(shell go env GOBIN))
GOBIN=$(root_dir)/bin
else
GOBIN=$(shell go env GOBIN)
endif

PKG?=$*
GO_TEST_ARGS ?= -race

# API generation utilities
CONTROLLER_GEN_VERSION ?= v0.19.0

# Architecture to use envtest with
ENVTEST_ARCH ?= amd64

# Kubernetes versions to use envtest with
ENVTEST_KUBERNETES_VERSION?=1.33

all: tidy generate fmt vet

tidy:
	$(MAKE) $(addprefix tidy-, $(MODULES))

tidy-%:
	cd $(subst :,/,$*); go mod tidy -compat=1.25

fmt:
	$(MAKE) $(addprefix fmt-, $(MODULES))

fmt-%:
	cd $(subst :,/,$*); go fmt ./...

vet:
	$(MAKE) $(addprefix vet-, $(MODULES))

vet-%:
	cd $(subst :,/,$*); go vet ./... ;\


# Run generate for all modules
generate:
	$(MAKE) $(addprefix generate-, $(MODULES))

# Generate manifests e.g. CRD, RBAC etc.
generate-%: controller-gen
	cd $(subst :,/,$*); CGO_ENABLED=0 $(CONTROLLER_GEN) schemapatch:manifests="./" paths="./..." ;\
	CGO_ENABLED=0 $(CONTROLLER_GEN) object:headerFile="$(root_dir)/hack/boilerplate.go.txt" paths="./..." ;\

# Run tests for all modules
test:
	$(MAKE) $(addprefix test-, $(MODULES))

# Run tests for a chunk of modules (usage: make test-chunk CHUNK=1/4)
# CHUNK format: N/M where N is the chunk number (1-indexed) and M is total chunks
CHUNK ?= 1/1
test-chunk:
	@CHUNK_NUM=$$(echo $(CHUNK) | cut -d'/' -f1); \
	TOTAL_CHUNKS=$$(echo $(CHUNK) | cut -d'/' -f2); \
	MODULES_LIST="$(MODULES)"; \
	TOTAL_MODULES=$$(echo $$MODULES_LIST | tr ' ' '\n' | wc -l); \
	CHUNK_SIZE=$$(( (TOTAL_MODULES + TOTAL_CHUNKS - 1) / TOTAL_CHUNKS )); \
	START_IDX=$$(( (CHUNK_NUM - 1) * CHUNK_SIZE + 1 )); \
	END_IDX=$$(( CHUNK_NUM * CHUNK_SIZE )); \
	if [ $$END_IDX -gt $$TOTAL_MODULES ]; then END_IDX=$$TOTAL_MODULES; fi; \
	CHUNK_MODULES=$$(echo $$MODULES_LIST | tr ' ' '\n' | sed -n "$${START_IDX},$${END_IDX}p" | tr '\n' ' '); \
	echo "Running tests for chunk $(CHUNK): modules $$START_IDX-$$END_IDX of $$TOTAL_MODULES"; \
	echo "Modules: $$CHUNK_MODULES"; \
	for mod in $$CHUNK_MODULES; do \
		$(MAKE) test-$$mod || exit 1; \
	done

# Run tests
KUBEBUILDER_ASSETS?="$(shell $(ENVTEST) --arch=$(ENVTEST_ARCH) use -i $(ENVTEST_KUBERNETES_VERSION) --bin-dir=$(ENVTEST_ASSETS_DIR) -p path)"
test-%: tidy-% generate-% fmt-% vet-% install-envtest
	cd $(subst :,/,$*); KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test ./... $(GO_TEST_ARGS) -coverprofile cover.out ;\

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
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION))

ENVTEST_ASSETS_DIR=$(shell pwd)/testbin
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

# Build fuzzers used by oss-fuzz.
fuzz-build:
	rm -rf $(shell pwd)/build/fuzz/
	mkdir -p $(shell pwd)/build/fuzz/out/

	docker build . --pull --tag local-fuzzing:latest -f tests/fuzz/Dockerfile.builder
	docker run --rm \
		-e FUZZING_LANGUAGE=go -e SANITIZER=address \
		-e CIFUZZ_DEBUG='True' -e OSS_FUZZ_PROJECT_NAME=fluxcd \
		-v "$(shell go env GOMODCACHE):/root/go/pkg/mod" \
		-v "$(shell pwd)/build/fuzz/out":/out \
		local-fuzzing:latest

# Run each fuzzer once to ensure they will work when executed by oss-fuzz.
fuzz-smoketest: fuzz-build
	docker run --rm \
		-v "$(shell pwd)/build/fuzz/out":/out \
		-v "$(shell pwd)/tests/fuzz/oss_fuzz_run.sh":/runner.sh \
		-e ENVTEST_BIN_VERSION=$(ENVTEST_KUBERNETES_VERSION) \
		local-fuzzing:latest \
		bash -c "/runner.sh"

# Prepare release for Go modules.
.PHONY: prep
prep: tools
	@./bin/flux-tools pkg prep

# Release Go modules.
.PHONY: release
release: tools
	@./bin/flux-tools pkg release

# Run vet for tools.
.PHONY: tools
tools:
	@cd cmd; \
	go mod tidy; \
	go fmt ./internal/... ./cli/...; \
	go vet ./internal/... ./cli/...; \
	go build -o ../bin/flux-tools ./cli
