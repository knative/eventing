# Conformance tests

Conformance tests verifies knative eventing implementation for expected behavior
described in
[specification](https://github.com/knative/eventing/tree/main/docs/spec).

## Running conformance tests

Run test with e2e tag and optionally select conformance test

> NOTE: Make sure you have built the
> [test images](https://github.com/knative/eventing/tree/main/test#building-the-test-images)!

```shell
ko apply -Rf test/config/
export SYSTEM_NAMESPACE=knative-eventing

go test -v -tags=e2e -count=1 ./test/conformance/...

go test -v -timeout 30s -tags e2e knative.dev/eventing/test/conformance -run ^TestMustPassTracingHeaders$

go test -v -timeout 30s -tags e2e knative.dev/eventing/test/conformance -run ^TestMustPassTracingHeaders$ --kubeconfig $KUBECONFIG
```

## Running Broker conformance tests against an existing broker

When developing a new broker, or testings against a preexisting broker setup you
can specify:

```shell
go test -v -tags e2e knative.dev/eventing/test/conformance -brokername=foo -brokernamespace=bar -run TestBrokerDataPlaneIngress

```

## Running conformance tests as a project admin

It is possible to run the conformance tests by a user with reduced privileges, e.g. project admin.
Some tests require cluster-admin privileges and those tests are excluded from execution in this case.
Running the conformance tests then consists of these steps:
1. The cluster admin creates test namespaces and required RBAC. Each test requires a separate namespace.
   By default, the namespace names consist of `eventing-e2e` prefix and numeric suffix starting from 0:
   `eventing-e2e0`, `eventing-e2e1`, etc. The prefix can be configured using the EVENTING_E2E_NAMESPACE env
  variable. There's a helper script in the current folder that will create all the required resources:
    ```shell
    NUM_NAMESPACES=40 ./create-namespace-rbac.sh
    ```
   Note: The number of required namespaces might grow over time.
1. The project admin runs the test suite with specific flags:
    ```shell
    go test -v -tags=e2e,project_admin -count=1 ./test/conformance \
      -reusenamespace \
      -kubeconfig=$PROJECT_ADMIN_KUBECONFIG
    ```
   The $PROJECT_ADMIN_KUBECONFIG's user is expected to be a project admin for all the
   created namespaces.
