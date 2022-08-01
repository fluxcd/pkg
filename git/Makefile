PROJECT_DIR = $(shell git rev-parse --show-toplevel)
# Base image used to build the Go binary
LIBGIT2_IMG ?= ghcr.io/fluxcd/golang-with-libgit2
LIBGIT2_TAG ?= v0.2.0

# Caches libgit2 versions per tag, "forcing" rebuild only when needed.
BUILD_DIR := $(shell pwd)/libgit2/build
LIBGIT2_PATH := $(BUILD_DIR)/libgit2/$(LIBGIT2_TAG)
LIBGIT2_LIB_PATH := $(LIBGIT2_PATH)/lib
LIBGIT2 := $(LIBGIT2_LIB_PATH)/libgit2.a

GO_TAGS ?=
GO_TEST_ARGS ?= -race

# Use $GOBIN from the environment if set, otherwise use ./bin
ifeq (,$(shell go env GOBIN))
GOBIN=$(PROJECT_DIR)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export CGO_ENABLED=1
export PKG_CONFIG_PATH=$(LIBGIT2_LIB_PATH)/pkgconfig
export CGO_LDFLAGS=$(shell PKG_CONFIG_PATH=$(PKG_CONFIG_PATH) pkg-config --libs --static --cflags libgit2 2>/dev/null)

tidy:
	go mod tidy -compat=1.17

fmt:
	go fmt ./...

vet: $(LIBGIT2)
	go vet ./...

test: $(LIBGIT2)
	go test -tags 'netgo,osusergo,static_build$(addprefix ,,$(GO_TAGS))' $(GO_TEST_ARGS) ./... -coverprofile cover.out

libgit2: $(LIBGIT2)  ## Detect or download libgit2 library

.PHONY: e2e
e2e: $(LIBGIT2)
	PKG_CONFIG_PATH="$(PKG_CONFIG_PATH)" CGO_LDFLAGS="$(CGO_LDFLAGS)" ./e2e/run.sh

COSIGN = $(GOBIN)/cosign
$(LIBGIT2):
	$(call go-install-tool,$(COSIGN),github.com/sigstore/cosign/cmd/cosign@latest)

	cd libgit2 && IMG=$(LIBGIT2_IMG) TAG=$(LIBGIT2_TAG) PATH=$(PATH):$(GOBIN) ./hack/install-libraries.sh

define go-install-tool
@[ -f $(1) ] || { \
set -e ;\
TMP_DIR=$$(mktemp -d) ;\
cd $$TMP_DIR ;\
go mod init tmp ;\
echo "Downloading $(2)" ;\
GOBIN=$(GOBIN) go install $(2) ;\
rm -rf $$TMP_DIR ;\
}
endef

