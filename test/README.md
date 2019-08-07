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
`e2e-*tests.sh` scripts under [`test`](.). After the tests finish, it will
delete the cluster._

### E2E tests

[`e2e-tests.sh`](./e2e-tests.sh) is the entry point for running all e2e tests.

You can run it simply with:

```shell
test/e2e-tests.sh
```

_By default, it will create a new GKE cluster in project `$PROJECT_ID`, start
Knative Serving and Eventing system, upload test images to `$KO_DOCKER_REPO`,
and run the end-to-end tests. After the tests finishes, it will delete the
cluster._

If you have already created your own Kubernetes cluster but haven't installed
Knative, you can run with `test/e2e-tests.sh --run-tests`.

If you have set up a running environment that meets
[the e2e test environment requirements](#environment-requirements), you can run
with `test/e2e-tests.sh --run-tests --skip-knative-setup`.

### Performance tests

[`performance-tests.sh`](./performance-tests.sh) is the entry point for running
all performance tests.

You can run it simply with:

```shell
test/performance-tests.sh
```

The steps and flags are exactly the same as e2e tests described above. After the
tests are done, test results named as `artifacts` will be saved under
`./performance`. You can also check the historic test result through
[testgrid](https://testgrid.knative.dev/eventing#performance).

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

```bash
go test -v -tags=e2e -count=1 ./test/e2e
```

By default, it will run all applicable tests against the cluster's default
`channel`.

If you want to run tests against other `channels`, you can specify them through
`-channels`.

```bash
go test -v -tags=e2e -count=1 ./test/e2e -channels=InMemoryChannel,KafkaChannel
```

By default, tests run against images with the `latest` tag. To override this
bevavior you can specify a different tag through `-tag`:

```bash
go test -v -tags=e2e -count=1 ./test/e2e -tag e2e
```

#### One test case

To run one e2e test case, e.g. `TestSingleBinaryEventForChannel`, use
[the `-run` flag with `go test`](https://golang.org/cmd/go/#hdr-Testing_flags):

```bash
go test -v -tags=e2e -count=1 ./test/e2e -run ^TestSingleBinaryEventForChannel$
```

By default, it will run the test against the default `channel`.

If you want to run it against another `channel`, you can specify it through
`-channels`.

```bash
go test -v -tags=e2e -count=1 ./test/e2e -run ^TestSingleBinaryEventForChannel$ -channels=InMemoryChannel
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
