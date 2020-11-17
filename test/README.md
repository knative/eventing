# Test

This directory contains tests and testing docs for `Knative Eventing`.

- [Unit tests](#running-unit-tests) reside in the codebase alongside the code
  they test
- [End-to-end tests](#running-end-to-end-tests) reside in [`/test/e2e`](./e2e)

## Running tests with scripts

### Presubmit tests

[`presubmit-tests.sh`](./presubmit-tests.sh) is the entry point for the tests
before code submission.

You can run it simply with:

```shell
test/presubmit-tests.sh
```

_By default, this script will run `build tests`, `unit tests` and
`integration tests`._ If you only want to run one type of tests, you can run
this script with corresponding flags like below:

```shell
test/presubmit-tests.sh --build-tests
test/presubmit-tests.sh --unit-tests
test/presubmit-tests.sh --integration-tests
```

_Note that if the tests you are running include `integration tests`, it will
create a new GKE cluster in project `$PROJECT_ID`, start Knative Serving and
Eventing system, upload test images to `$KO_DOCKER_REPO`, and run all
`e2e-*tests.sh` scripts under [`test`](.)._

### E2E tests

[`e2e-tests.sh`](./e2e-tests.sh) is the entry point for running all e2e tests.

In this section we use GCP as an example, other platforms might need different
options.

You can run it simply with:

```shell
test/e2e-tests.sh --gcp-project-id=$PROJECT_ID
```

_By default, it will create a new GKE cluster in project `$PROJECT_ID`, start
Knative Serving and Eventing system, upload test images to `$KO_DOCKER_REPO`,
and run the end-to-end tests. After the tests finishes, it will delete the
cluster._

If you have already created your own Kubernetes cluster but haven't installed
Knative, you can run with
`test/e2e-tests.sh --run-tests --gcp-project-id=$PROJECT_ID`.

If you have set up a running environment that meets
[the e2e test environment requirements](#environment-requirements), you can run
with
`test/e2e-tests.sh --run-tests --skip-knative-setup --gcp-project-id=$PROJECT_ID`.

## Running tests with `go test` command

### Running unit tests

You can also use `go test` command to run unit tests:

```shell
go test -v ./pkg/...
```

_By default `go test` will not run [the e2e tests](#running-end-to-end-tests),
which needs [`-tags=e2e`](#running-end-to-end-tests) to be enabled._

### Running end-to-end tests

To run [the e2e tests](./e2e) with `go test` command, you need to have a running
environment that meets
[the e2e test environment requirements](#environment-requirements), and you need
to specify the build tag `e2e`.

If you are using a private registry that will require authentication then you'll
need to create a Secret in your `default` Namespace called
`kn-eventing-test-pull-secret` with the Docker login credentials. This Secret
will then be copied into any new Namespace that is created by the testing
infrastructure, and linked to any ServiceAccount created as a imagePullSecret.
Note: some tests will use the `knative-eventing-injection` label to
automatically create new ServiceAccounts in some Namespaces, this feature does
not yet support private registries. See
https://github.com/knative/eventing/issues/1862 for status of this issue.

```bash
go test -v -tags=e2e -count=1 ./test/e2e
```

By default, tests run against images with the `latest` tag. To override this
behavior you can specify a different tag through `-tag`:

```bash
go test -v -tags=e2e -count=1 ./test/e2e -tag e2e
```

#### One test case

To run one e2e test case, e.g. `TestSingleBinaryEventForChannel`, use
[the `-run` flag with `go test`](https://golang.org/cmd/go/#hdr-Testing_flags):

```bash
go test -v -tags=e2e -count=1 ./test/e2e -run ^TestSingleBinaryEventForChannel$
```

## Environment requirements

There's couple of things you need to install before running e2e tests locally.

1. A running [Knative](https://www.knative.dev/docs/install/) cluster
1. A docker repo containing [the test images](#test-images)

## Test images

### Building the test images

_Note: this is only required when you run e2e tests locally with `go test`
commands. Running tests through e2e-tests.sh will publish the images
automatically._

The [`upload-test-images.sh`](./upload-test-images.sh) script can be used to
build and push the test images used by the e2e tests. It requires:

- [`KO_DOCKER_REPO`](https://github.com/knative/serving/blob/master/DEVELOPMENT.md#environment-setup)
  to be set
- You to be
  [authenticated with your `KO_DOCKER_REPO`](https://github.com/knative/serving/blob/master/DEVELOPMENT.md#environment-setup)
- [`docker`](https://docs.docker.com/install/) to be installed

To run the script for all end to end test images:

```bash
./test/upload-test-images.sh
```

For images deployed in GCR, a docker tag is mandatory to avoid issues with using
`latest` tag:

```bash
./test/upload-test-images.sh e2e
```

### Adding new test images

New test images should be placed in `./test/test_images`. For each image create
a new sub-folder and include a Go file that will be an entry point to the
application. This Go file should use the package `main` and include the function
`main()`. It is a good practice to include a `readme` file as well. When
uploading test images, `ko` will build an image from this folder.

## Flags

Flags are similar to those in
[`Knative Serving`](https://github.com/knative/serving/blob/master/test/README.md#flags-1).
