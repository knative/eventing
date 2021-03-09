# Reconciler Tests

This is the staging location for the new
[e2e testing framework](https://github.com/knative-sandbox/reconciler-test).

To run the tests on an existing cluster:

```bash
SYSTEM_NAMESPACE=knative-eventing go test -count=1 -v -tags=e2e ./test/rekt/...
```

To run just one test:

```bash
SYSTEM_NAMESPACE=knative-eventing go test -count=1 -v -tags=e2e -run Smoke_PingSource ./test/rekt/...
```
