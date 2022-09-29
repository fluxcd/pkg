PROJECT_DIR = $(shell git rev-parse --show-toplevel)

LIBGIT2_BUILD_DIR := $(shell pwd)/build
# Include and export all libgit2 related setup variables before proceeding.
include ./libgit2-vars.env
export

GO_TAGS ?=
GO_TEST_ARGS ?= -race

# Defines whether cosign verification should be skipped.
SKIP_COSIGN_VERIFICATION ?= false

# Use $GOBIN from the environment if set, otherwise use ./bin
ifeq (,$(shell go env GOBIN))
GOBIN=$(PROJECT_DIR)/bin
else
GOBIN=$(shell go env GOBIN)
endif

export CGO_ENABLED=1
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

COSIGN = $(GOBIN)/cosign
$(LIBGIT2):
	$(call go-install-tool,$(COSIGN),github.com/sigstore/cosign/cmd/cosign@latest)

	IMG=$(LIBGIT2_IMG) TAG=$(LIBGIT2_TAG) PATH=$(PATH):$(GOBIN) ./hack/install-libraries.sh

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

