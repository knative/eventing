# Event Sources

## Specification

This
[document](https://github.com/knative/eventing/blob/main/docs/source/README.md)
contains the source specification.

## Implementations

There are various ways to implement Event Sources, as described in the
[documentation](https://github.com/knative/docs/blob/main/code-samples/eventing/writing-event-source-easy-way/README.md).

### Receive Adapters

Receive adapter is a reusable pattern for implementing sources.

There is a
[tutorial](https://knative.dev/docs/eventing/custom-event-source/custom-event-source/receive-adapter/) on
how to implement single-resource receive adapter.

This [document](./receive-adapter.md) provides a deep-dive on receive adapter.

### SinkBinding

The documentation contains an
[example](https://knative.dev/docs/eventing/samples/sinkbinding/) on how to use
`SinkBinding` to implement an event source.

### ContainerSource

The documentation contains an
[example](https://knative.dev/docs/eventing/custom-event-source/containersource/reference/) on how to
use `ContainerSource` to implement an event source.
