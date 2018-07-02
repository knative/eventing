# Motivation

Knative Eventing implements the common components for an event delivery
ecosystem. These common components enable producing and consuming events,
adhering to the [CloudEvents
Specification](https://github.com/cloudevents/spec), in a decoupled way.

Kubernetes has no primitives related to event processing, yet this is an
essential component in serverless workloads. Eventing introduces high-level
primitives for event production and delivery with a focus on push over HTTP. If a
new event source or type is required of your application, the effort required
to plumb them into the existing eventing framework will be minimal and will
integrate with existing message consumers.

The Knative Eventing APIs is intended to operate independently, and interop
well with the [Compute API](https://github.com/knative/serving) and [Build
API](https://github.com/knative/build).
