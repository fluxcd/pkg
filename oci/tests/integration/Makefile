GO_TEST_ARGS ?=
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
	TEST_IMG=$(TEST_IMG) go test -timeout $(TEST_TIMEOUT) -v ./ $(GO_TEST_ARGS) $(PROVIDER_ARG) --tags=integration

test-aws:
	$(MAKE) test PROVIDER_ARG="-provider aws"

test-azure:
	$(MAKE) test PROVIDER_ARG="-provider azure"

test-gcp:
	$(MAKE) test PROVIDER_ARG="-provider gcp"
