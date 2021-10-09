VER?=0.0.1
MODULES=$(shell find . -mindepth 2 -maxdepth 4 -type f -name 'go.mod' | cut -c 3- | sed 's|/[^/]*$$||' | sort -u | tr / :)
targets=$(addprefix test-, $(MODULES))
root_dir=$(dir $(abspath $(lastword $(MAKEFILE_LIST))))
ENVTEST_BIN_VERSION?=latest
KUBEBUILDER_ASSETS?="$(shell $(SETUP_ENVTEST) use -i $(ENVTEST_BIN_VERSION) -p path)"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all:
	$(MAKE) $(targets)

tidy-%:
	cd $(subst :,/,$*); go mod tidy

fmt-%:
	cd $(subst :,/,$*); go fmt ./...

vet-%:
	cd $(subst :,/,$*); go vet ./...

generate-%: controller-gen
	# Run schemapatch to validate all the kubebuilder markers before generation.
	cd $(subst :,/,$*); $(CONTROLLER_GEN) schemapatch:manifests="./" paths="./..."
	cd $(subst :,/,$*); $(CONTROLLER_GEN) object:headerFile="$(root_dir)/hack/boilerplate.go.txt" paths="./..."

test-%: generate-% tidy-% fmt-% vet-% setup-envtest
	cd $(subst :,/,$*); KUBEBUILDER_ASSETS=$(KUBEBUILDER_ASSETS) go test ./... -coverprofile cover.out

release-%:
	$(eval REL_PATH=$(subst :,/,$*))
	@if ! test -f $(REL_PATH)/go.mod; then echo "Missing ./$(REL_PATH)/go.mod, terminating release process"; exit 1; fi
	git checkout main
	git pull
	git tag "$(REL_PATH)/v$(VER)"
	git push origin "$(REL_PATH)/v$(VER)"

# Find or download controller-gen
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.5.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

# Find or download setup-envtest
setup-envtest:
ifeq (, $(shell which setup-envtest))
	@{ \
	set -e ;\
	SETUP_ENVTEST_TMP_DIR=$$(mktemp -d) ;\
	cd $$SETUP_ENVTEST_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-runtime/tools/setup-envtest@latest ;\
	rm -rf $$SETUP_ENVTEST_TMP_DIR ;\
	}
SETUP_ENVTEST=$(GOBIN)/setup-envtest
else
SETUP_ENVTEST=$(shell which setup-envtest)
endif
