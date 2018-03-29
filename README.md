# Eventing

This repo contains a work-in-progress eventing system that is designed to
address a common need for cloud native development:

1. Services are loosely coupled during development and deployed independently
2. A producer can generate events before a consumer is listening, and a consumer
can express an interest in an event or class of events that is not yet being
produced.
3. Services can be connected to create new applications
    - without modifying producer or consumer.
    - with the ability to select a specific subset of events from a particular
    producer

The above concerns are consistent with the [design goals](https://github.com/cloudevents/spec/blob/master/spec.md#design-goals) of CloudEvents, a common specification for cross-service interoperability being developed by the CNCF Serverless WG.

## System design notes

We’re following an agile process, starting from a working prototype that
addresses a single use case and iterating to support additional use cases.

This repo depends on [elafros/elafros](https://github.com/elafros/elafros) and
together with [elafros/build](https://github.com/elafros/build) provides a
complete serverless platform.

The primary goal of *eventing* is interoperabilty; therefore, we expect to
provide common libraries that can be used in other systems to emit or consume
events.


## Naming

We'll be tracking the CloudEvents nomenclature as much as possible; however,
that is in flux and many of the needed terms are outside of the current scope
of the specification. At the moment, in our code and documentation, we may use
the terms "binding" and "flow" at different times to represent the concept of
attaching an event (or filtered event stream via a "trigger) to an action.

# Getting Started

* [Setup Elafros](https://github.com/elafros/elafros)
* [Setup Binding](./DEVELOPMENT.md)
* [Run samples](./sample/README.md)

