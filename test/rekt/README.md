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

## Broker tests.

The Broker class can be overridden by using the envvar `BROKER_CLASS`. By
default, this will be `MTChannelBasedBroker`.

The Broker templates can be overridden by using the env var `BROKER_TEMPLATES`.

```bash
BROKER_CLASS=MyCustomBroker
BROKER_TEMPLATES=/path/to/custom/templates
SYSTEM_NAMESPACE=knative-eventing \
  go test -count=1 -v -tags=e2e -run Smoke_PingSource ./test/rekt/...
```

### Custom templates

The minimum shape of a custom template for namespaced resources:

```yaml
apiVersion: rando.api/v1
kind: MyResource
metadata:
  name: { { .name } }
  namespace: { { .namespace } }
spec:
  any:
    thing: that
  is: required
```

See [./resources](./resources) for examples.
