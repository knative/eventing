# Motivation

The goal of Knative Eventing is to define common, composable primitives to
enable late-binding event sources and event consumers.

<!-- TODO(n3wscott): [Why late-binding] -->

Knative Eventing has following principles:

1. Services are loosely coupled during development and deployed independently on
   a variety of platforms (Kubernetes, VMs, SaaS or FaaS).

1. A producer can generate events before a consumer is listening, and a consumer
   can express an interest in an event or class of events that is not yet being
   produced.

1. Services can be connected to create new applications
   - without modifying producer or consumer.
   - with the ability to select a specific subset of events from a particular
     producer

These primitives enable producing and consuming events adhering to the
[CloudEvents Specification](https://github.com/cloudevents/spec), in a decoupled
way.

Kubernetes has no primitives related to event processing, yet this is an
essential component in serverless workloads. Eventing introduces high-level
primitives for event production and delivery with an initial focus on push over
HTTP. If a new event source or type is required of your application, the effort
required to plumb them into the existing eventing framework will be minimal and
will integrate with CloudEvents middleware and message consumers.

Knative eventing implements common components of an event delivery ecosystem:
enumeration and discovery of event sources, configuration and management of
event transport, and declarative binding of events (generated either by storage
services or earlier computation) to further event processing and persistence.

The Knative Eventing API is intended to operate independently, and interoperate
well with the [Serving API](https://github.com/knative/serving) and
[Build API](https://github.com/knative/build).

---

_Navigation_:

- **Motivation and goals**
- [Resource type overview](overview.md)
- [Interface contracts](interfaces.md)
- [Object model specification](spec.md)
