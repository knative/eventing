# Event Sources

## Specification

This
[document](https://github.com/knative/specs/blob/main/specs/eventing/sources.md)
contains the source specification.

## Implementations

There are various ways to implement Event Sources, as described in the
[documentation](https://knative.dev/docs/eventing/samples/writing-event-source/).

### Receive Adapters

Receive adapter is a reusable pattern for implementing sources.

There is a
[tutorial](https://knative.dev/docs/eventing/samples/writing-event-source/) on
how to implement single-resource receive adapter.

This [document](./receive-adapter.md) provides a deep-dive on receive adapter.

### SinkBinding

The documentation contains an
[example](https://knative.dev/docs/eventing/samples/sinkbinding/) on how to use
`SinkBinding` to implement an event source.

### ContainerSource

The documentation contains an
[example](https://knative.dev/docs/eventing/samples/container-source/) on how to
use `ContainerSource` to implement an event source.
