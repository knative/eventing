# Test

This directory contains tests and testing docs.

* [Unit tests](#running-unit-tests) currently reside in the codebase alongside the code they test
* [End-to-end tests](#running-end-to-end-tests)


## Running unit tests

Use `go test`:

```shell
go test -v ./pkg/...
```

## Running end-to-end tests

### Dependencies

There's couple of things you need to install before running e2e tests locally.

```shell
go get -u k8s.io/test-infra/kubetest
```

Simply run the `e2e-tests.sh` script. It will create a GKE cluster, install Knative Serving
stack with Istio and run the end-to-end tests against the Knative Eventing built from source.

If you already have the `*_OVERRIDE` environment variables set, call
the script with the `--run-tests arguments` and it will use the cluster
and run the tests. Note that this requires you to have Serving and Istio
installed and configured to your particular configuration setup. Knative Eventing will
still built and deployed from source.

Otherwise, calling this script without arguments will create a new cluster in
project `$PROJECT_ID`, start Elafros and the eventing system, run the
tests and delete the cluster. In this case, it's required that `$KO_DOCKER_REPO`
point to a valid writable docker repo.
