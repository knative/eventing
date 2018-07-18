# Knative Eventing

This repository contains a work-in-progress eventing system that is designed to
address a common need for cloud native development:

1. Services are loosely coupled during development and deployed independently
2. A producer can generate events before a consumer is listening, and a consumer
can express an interest in an event or class of events that is not yet being
produced.
3. Services can be connected to create new applications
    - without modifying producer or consumer, and
    - with the ability to select a specific subset of events from a particular
    producer.

For complete Knative Eventing documentation, see [Knative Eventing](https://github.com/knative/docs/tree/master/eventing) or [Knative docs](https://github.com/knative/docs/) to learn about Knative.
