# OCI integration test

OCI integration test uses a test application(`testapp/`) to test the
oci package against each of the supported cloud providers.

**NOTE:** Tests in this package aren't run automatically by the `test-*` make
target at the root of `fluxcd/pkg` repo. These tests are more complicated than
the regular go tests as it involves cloud infrastructure and have to be run
explicitly.

Before running the tests, build the test app with `make docker-build` and use
the built container application in the integration test.

The integration test provisions cloud infrastructure in a target provider and
runs the test app as a batch job which tries to log in and list tags from the
test registry repository. A successful job indicates successful test. If the job
fails, the test fails.

Logs of a successful job run:
```console
$ kubectl logs test-job-93tbl-4jp2r
2022/07/28 21:59:06 repo: xxx.dkr.ecr.us-east-2.amazonaws.com/test-repo-flux-test-heroic-ram
1.659045546831094e+09   INFO    logging in to AWS ECR for xxx.dkr.ecr.us-east-2.amazonaws.com/test-repo-flux-test-heroic-ram
2022/07/28 21:59:06 logged in
2022/07/28 21:59:06 tags: [v0.1.4 v0.1.3 v0.1.0 v0.1.2]
```

## Requirements

### Amazon Web Services

- AWS account with access key ID and secret access key with permissions to
    create EKS cluster and ECR repository.
- AWS CLI, does not need to be configured with the AWS account.
- Docker CLI for registry login.
- kubectl for applying certain install manifests.

### Microsoft Azure

- Azure account with an active subscription to be able to create AKS and ACR,
    and permission to assign roles. Role assignment is required for allowing AKS
    workloads to access ACR.
- Azure CLI, need to be logged in using `az login`.
- Docker CLI for registry login.
- kubectl for applying certain install manifests.

### Google Cloud Platform

- GCP account with project and GKE, GCR and Artifact Registry services enabled
    in the project.
- gcloud CLI, need to be logged in using `gcloud auth login`.
- Docker CLI for registry login.
- kubectl for applying certain install manifests.

**NOTE:** Unlike ECR, AKS and Google Artifact Registry, Google Container
Registry tests don't create a new registry. It pushes to an existing registry
host in a project, for example `gcr.io`. Due to this, the test images pushed to
GCR aren't cleaned up automatically at the end of the test and have to be
deleted manually.

## Test setup

Copy `.env.sample` to `.env`, put the respective provider configurations in the
environment variables and source it, `source .env`.

Ensure the test app container image is built and ready for testing. Test app
container image can be built with make target `docker-build`.

Run the test with `make test-*`, setting the test app image with variable
`TEST_IMG`. By default, the default test app container image,
`fluxcd/testapp:test`, will be used.

```console
$ make test-azure
make test PROVIDER_ARG="-provider azure"
docker image inspect fluxcd/testapp:test >/dev/null
TEST_IMG=fluxcd/testapp:test go test -timeout 30m -v ./ -verbose -retain -provider azure --tags=integration
2022/07/29 02:06:51 Terraform binary:  /usr/bin/terraform
2022/07/29 02:06:51 Init Terraform
...
```

## Debugging the tests

For debugging environment provisioning, enable verbose output with `-verbose`
test flag.

```console
$ make test-aws GO_TEST_ARGS="-verbose"
```

The test environment is destroyed at the end by default. Run the tests with
`-retain` flag to retain the created test infrastructure.

```console
$ make test-aws GO_TEST_ARGS="-retain"
```

The tests require the infrastructure state to be clean. For re-running the tests
with a retained infrastructure, set `-existing` flag.

```console
$ make test-aws GO_TEST_ARGS="-retain -existing"
```

To delete an existing infrastructure created with `-retain` flag:

```console
$ make test-aws GO_TEST_ARGS="-existing"
```
