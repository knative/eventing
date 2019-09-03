# Conformance tests

Conformance tests verifies kantive eventing implementation for expected behavior described in
[specification](https://github.com/knative/eventing/tree/master/docs/spec).

## Running performance tests

Run test with e2e tag and optionally select conformance test

```go
go test -v -tags=e2e -count=1 ./test/conformance/...

go test -v -timeout 30s -tags e2e knative.dev/eventing/test/conformance -run ^TestMustPassTracingHeaders$

go test -v -timeout 30s -tags e2e knative.dev/eventing/test/conformance -run ^TestMustPassTracingHeaders$ --kubeconfig $KUBECONFIG
```
