GO_TEST_ARGS ?=
PROVIDER_ARG ?=
TEST_TIMEOUT ?= 30m
BUILD_ARGS ?= --load

TEST_IMG ?= fluxcd/testapp:test

.PHONY: app
app:
	CGO_ENABLED=0 GOOS=linux go build -v -o app ./testapp

docker-build: app
	docker buildx build ${BUILD_ARGS} -t $(TEST_IMG) .

test:
	docker image inspect $(TEST_IMG) >/dev/null
	TEST_IMG=$(TEST_IMG) go test -timeout $(TEST_TIMEOUT) -v ./ $(GO_TEST_ARGS) $(PROVIDER_ARG) --tags=integration

test-aws:
	$(MAKE) test PROVIDER_ARG="-provider aws"

test-azure:
	$(MAKE) test PROVIDER_ARG="-provider azure"

test-gcp:
	$(MAKE) test PROVIDER_ARG="-provider gcp"
