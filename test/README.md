# Test

This directory contains tests and testing docs for `Knative Eventing`.

- [Unit tests](#running-unit-tests) reside in the codebase alongside the code
  they test
- [End-to-end tests](#running-end-to-end-tests) reside in [`/test/e2e`](./e2e)

## Running unit tests

Use `go test`:

```shell
go test -v ./pkg/...
```

_By default `go test` will not run [the e2e tests](#running-end-to-end-tests),
which need [`-tags=e2e`](#running-end-to-end-tests) to be enabled._

## Presubmit tests

[`presubmit-tests.sh`](./presubmit-tests.sh) is the entry point for the
[end-to-end tests](/test/e2e).

This script, and consequently, the e2e tests will be run before every code
submission. You can run these tests manually with:

```shell
test/presubmit-tests.sh
```

_Note that to run `presubmit-tests.sh` or `e2e-tests.sh` scripts, you need to
have a running environment that meets
[the e2e test environment requirements](#environment-requirements)_

## Running end-to-end tests

To run [the e2e tests](./e2e), you need to have a running environment that meets
[the e2e test environment requirements](#environment-requirements), and you need
to specify the build tag `e2e`.

```bash
go test -v -tags=e2e -count=1 ./test/e2e
```

### One test case

To run one e2e test case, e.g. TestKubernetesEvents, use
[the `-run` flag with `go test`](https://golang.org/cmd/go/#hdr-Testing_flags):

```bash
go test -v -tags=e2e -count=1 ./test/e2e -run ^TestKubernetesEvents$
```

### Environment requirements

There's couple of things you need to install before running e2e tests locally.

1. `kubetest` installed:

   ```bash
   go get -u k8s.io/test-infra/kubetest
   ```

1. [A running `Knative Serving` cluster.]
1. A docker repo containing [the test images](#test-images)

Simply run the `./test/e2e-tests.sh` script. It will create a GKE cluster,
install Knative Serving stack with Istio, upload test images to your Docker repo
and run the end-to-end tests against the Knative Eventing built from source.

If you already have the `*_OVERRIDE` environment variables set, call the script
with the `--run-tests` argument and it will use the cluster and run the tests.
Note that this requires you to have Serving and Istio installed and configured
to your particular configuration setup. Knative Eventing will still built and
deployed from source.

Otherwise, calling this script without arguments will create a new cluster in
project `$PROJECT_ID`, start Knative Serving and the eventing system, upload
test images, run the tests and delete the cluster. In this case, it's required
that `$KO_DOCKER_REPO` points to a valid writable docker repo.

## Test images

### Building the test images

Note: this is only required when you run e2e tests locally with `go test`
commands. Running tests through e2e-tests.sh will publish the images
automatically.

The [`upload-test-images.sh`](./upload-test-images.sh) script can be used to
build and push the test images used by the e2e tests. It requires:

- [`KO_DOCKER_REPO`](https://github.com/knative/serving/blob/master/DEVELOPMENT.md#environment-setup)
  to be set
- You to be
  [authenticated with your `KO_DOCKER_REPO`](https://github.com/knative/serving/blob/master/DEVELOPMENT.md#environment-setup)
- [`docker`](https://docs.docker.com/install/) to be installed

To run the script for all end to end test images:

```bash
./test/upload-test-images.sh e2e
```

A docker tag is mandatory to avoid issues with using `latest` tag for images
deployed in GCR.

### Adding new test images

New test images should be placed in `./test/test_images`. For each image create
a new sub-folder and include a Go file that will be an entry point to the
application. This Go file should use the package "main" and include the function
main(). It is a good practice to include a readme file as well. When uploading
test images, `ko` will build an image from this folder.

## Flags

Flags are similar to those in
[`Knative Serving`](https://github.com/knative/serving/blob/master/test/README.md#flags-1)
