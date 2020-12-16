# Conformance tests

Conformance tests verifies knative eventing implementation for expected behavior
described in
[specification](https://github.com/knative/eventing/tree/master/docs/spec).

## Running performance tests

Run test with e2e tag and optionally select conformance test

> NOTE: Make sure you have built the
> [test images](https://github.com/knative/eventing/tree/master/test#building-the-test-images)!

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
go test -v -tags e2e knative.dev/eventing/test/conformance -brokername=foo -brokernamespace=bar -run TestBrokerV1Beta1DataPlaneIngress

```
