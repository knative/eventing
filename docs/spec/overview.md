# Resource Groups

Knative Eventing API is grouped into _Channels_ and _Feeds_:

* _Channels_ define an abstraction between the eventing infrastructure and the
  consumption and production of the events.

* _Feeds_ bridge the event source into the eventing framework.

* _Flows_ are the higher order abstractions for eventing that use _Channels_
  and _Feeds_.

Typically, _Feeds_ perform out-of-cluster provisioning, while _Channels_ are
within-cluster management of event delivery.

# Resource Types

The primary resources in the Knative Eventing _Feeds_ API are EventSource,
EventType and Feed:

* **EventSource**, an archetype of an event producer.

* **EventType**, the schema for an event.

* **Feed**, the association between the output of an event producer to the
  input of an event consumer.


The primary resources in the Knative Eventing _Channels_ API are Channel,
Subscription and Bus:

* **Channel**, a named endpoint which accepts and forwards events.

* **Bus**, an implementation of an event delivery mechanism.

* **Subscription**, an expressed interest in events to be delivered to a
  service.

The primary resources in the Knative Eventing _Flows_ API are Flow:

* **Flow**, the connection between an event producer to the event consumer.

![Object Model](images/overview-reference.png)

## EventSource

A software system which wishes to make changes in state discoverable via
eventing, without prior knowledge of systems which might consume state changes.

## EventType

TODO

## Feed

A configuration of event transmission from a single Source to a single Action.
A Feed may contain additional properties of the event transmission such as
filtering, timeouts, rate limits, and buffering.

## Channel

A named in-cluster Service on a Bus which accepts events delivered from a Feed.

## Bus

A system which routes events from Channels to subscribed Service endpoints. The
Broker MAY (RFC 2119) provide durable, at-least-once semantics and buffering of
events between the Source and the Action. Brokers also perform event fan-out
across multiple actions which are subscribed to the same channel.


## Subscription

A control plane construct which allows the Bus to route events received on a
Channel to the subscribed Service.

# Eventing

TODO an overview of how these objects are created and perhaps the persona
expected to make each.
