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

Simply run the `e2e-tests.sh` script. It will run the end-to-end tests against the
eventing system built from source.

If you already have the `*_OVERRIDE` environment variables set, call
the script with the `--run-tests arguments` and it will use the cluster
and run the tests.

Otherwise, calling this script without arguments will create a new cluster in
project `$PROJECT_ID`, start Elafros and the eventing system, run the
tests and delete the cluster. In this case, it's required that `$KO_DOCKER_REPO`
point to a valid writable docker repo.
