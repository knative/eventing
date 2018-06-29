# Motivation

The goal of the Knative Eventing project is to provide a common toolkit and API
framework for producing and consuming events, primarily inside of serverless
workloads.

Kubernetes has no primitives related to event processing, yet this is an
essential component in serverless workloads. Eventing will introduce high-level
primitives for event production and delivery in forms that are extensible. If a
new event source or type is required of your application, the effort required
to plumb them into the existing eventing framework will be minimal and will
integrate with existing message consumers.

The Knative Eventing APIs consist of Eventing API (these documents),
[Compute API](https://github.com/knative/serving) and
[Build API](https://github.com/knative/build).