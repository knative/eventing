# Resource Groups

Knative Eventing API is grouped into _Channels_ and _Feeds_:

* _Channels_ define an abstraction between the eventing infrastructure and the
  consumption and production of the events.

* _Feeds_ bridge the event source into the eventing framework.

Typically, _Feeds_ perform out-of-cluster provisioning, while _Channels_ are
within-cluster management of event delivery.

# Resource Types

The primary resources in the Knative Eventing _Feeds_ API are EventSource, EventType and Bind:

* **EventSource**, an archetype of an event producer.

* **EventType**, the schema for an event.

* **Bind**, ties the output of an event producer to the input of an event
  consumer.


The primary resources in the Knative Eventing _Channels_ API are Channel, Subscription and Bus:

* **Channel**, a named endpoint which accepts and forwards events.

* **Bus**, an implementation of an event delivery mechanism.

* **Subscription**, an expressed interest in events to be delivered to a
  service.

![TODO:: Object model](images/object_model.png)


## EventSource

A software system which wishes to make changes in state discoverable via
eventing, without prior knowledge of systems which might consume state changes.

## EventType

TODO

## Bind

A configuration of event transmission from a single Source to a single Action.
A Binding may contain additional properties of the event transmission such as
filtering, timeouts, rate limits, and buffering.

## Channel

A named endpoint on a Bus which accepts events delivered from an outside system.

## Bus

A system which routes events from Channels to Actions. The Broker MAY (RFC
2119) provide durable, at-least-once semantics and buffering of events between
the Source and the Action. Brokers also perform event fan-out across multiple
actions which are subscribed to the same channel.


## Subscription

A control plane construct which routes events received on a Channel to a URL or
DNS name

# Eventing

TODO an overview of how these objects are created and perhaps the persona
expected to make each.
