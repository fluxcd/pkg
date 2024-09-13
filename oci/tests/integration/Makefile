GO_TEST_ARGS ?=
GO_TEST_PREFIX ?=
PROVIDER_ARG ?=
TEST_TIMEOUT ?= 30m
GOARCH ?= amd64
GOOS ?= linux

TEST_IMG ?= fluxcd/testapp:test

.PHONY: app
app:
	CGO_ENABLED=0 GOARCH=$(GOARCH) GOOS=$(GOOS) go build -v -o app ./testapp

docker-build: app
	docker buildx build -t $(TEST_IMG) --load .

test:
	docker image inspect $(TEST_IMG) >/dev/null
	TEST_IMG=$(TEST_IMG) go test -timeout $(TEST_TIMEOUT) -v ./ -run "^$(GO_TEST_PREFIX).*" $(GO_TEST_ARGS) $(PROVIDER_ARG) --tags=integration

test-aws:
	$(MAKE) test PROVIDER_ARG="-provider aws"

test-azure:
	$(MAKE) test PROVIDER_ARG="-provider azure"

test-azure-git:
	$(MAKE) test PROVIDER_ARG="-provider azure" GO_TEST_PREFIX="TestGit"

test-azure-oci:
	$(MAKE) test PROVIDER_ARG="-provider azure" GO_TEST_PREFIX="TestOci"

test-gcp:
	$(MAKE) test PROVIDER_ARG="-provider gcp"

destroy:
	go test -timeout $(TEST_TIMEOUT) -v ./ $(GO_TEST_ARGS) $(PROVIDER_ARG) -destroy-only --tags=integration

destroy-aws:
	$(MAKE) destroy PROVIDER_ARG="-provider aws"

destroy-azure:
	$(MAKE) destroy PROVIDER_ARG="-provider azure"

destroy-gcp:
	$(MAKE) destroy PROVIDER_ARG="-provider gcp"
