# Resource Groups

Knative Eventing API is grouped into _Channels_ and _Feeds_:

* _Channels_ define an abstraction between the eventing infrastructure and the
  consumption and production of the events.

* _Feeds_ bridge the event source into the eventing framework.

# Resource Types

The primary resources in the Knative Eventing _Feeds_ API are EventSource, EventType and Bind:

* An **EventSource** wraps an event producer and is able to produce multiple definitions of

* **EventType**, which describes the schema for the shape of the event and Subscription. 

* A **Bind** connects an EventType to a Channel or Route.


The primary resources in the Knative Eventing _Channels_ API are Channel, Subscription and Bus:

* A **Channel** is a logical endpoint to allow events to flow into a

* **Bus**, which implements delivery mechanisms allowing events to flow to a

* **Subscription**, which allows events of interest to be delivered to a
  service.


![TODO:: Object model](images/object_model.png)


## EventSource

TODO

## EventType

TODO

## Bind

TODO

## Channel

TODO

## Bus

TODO

## Subscription

TODO

# Eventing

TODO an overview of how these objects are created and perhaps the persona
expected to make each.
