# Overview

In the serverless model, computation and storage can be intertwined by having
storage and compute systems publish events which are handled asynchronously by
further compute services. Event delivery provides common infrastructure for
decoupling event producers and consumers such that the linking between them can
be performed without change to either. Loosely coupled eventing systems enable
composable services which retain the ability to scale independently.
Declarative event binding reduces the effort to create a scalable system that
combines managed services and custom code.

Knative eventing implements common components of an event delivery ecosystem:
enumeration and discovery of event sources, configuration and management of
event transport, and declarative binding of events (generated either by storage
services or earlier computation) to further event processing and persistence.

The following is a detailed specification for the `eventing.knative.dev` API
surface, which provides control mechanism for Knative eventing.


# Resource Types

The eventing API defines several resource types as well as interfaces which may
be fulfilled by other Kubernetes objects ([Callable](interfaces.md#callable),
[Subscribable](interfaces.md#subscribable),
[Channelable](interfaces.md#channelable),
[Targetable](interfaces.md#targetable)). The API defines and provides a
complete implementation for [Subscription](spec.md#kind-subscription), and
abstract resource definitions for [Sources](spec.md#kind-source),
[Channels](spec.md#kind-channel), and [Providers](spec.md#kind-provisioner)
which may be fulfilled by multiple backing implementations (much like the
Kubernetes Ingress resource).

 * A **Subscription** describes the transformation of an event (via a
   _Callable_) and optional storage of a returned event. 

 * A **Source** represents an ongoing selection of events from an external
   system. A Subscription is used to connect these events to subsequent
   processing steps.

 * A **Channel** provides event storage and fanout of events from a well-known
   input address to multiple outputs described by Subscriptions. 

<!-- This image is sourced from https://drive.google.com/open?id=10mmXzDb8S_4_ZG_hcBr7s4HPISyBqcqeJLTXLwkilRc -->
![Resource Types Overview](images/resource-types-overview.svg)

 * **Provisioners** act as a catalog of the types of Sources and Channels
   currently active in the eventing system.

<!-- This image is sourced from https://drive.google.com/open?id=1o_0Xh5VjwpQ7Px08h_Q4qnaOdMjt4yCEPixRFwJQjh8 -->
![Resource Types Provisioners](images/resource-types-provisioner.svg)

## Subscription

**Subscriptions** define the transport of events from one storage location
(typically, **Source** or **Channel**) to the next (typically, a **Channel**)
through optional transformations (such as a Knative Service which processes
CloudEvents over HTTP). A **Subscription** resolves the addresses of
transformations (`call`) and destination storage (`result`) through the
_Callable_ and _Sinkable_ interface contracts, and writes the resolved
addresses to the _Subscribable_ `from` resource. A **Subscriptions** do not
need to specify both a transformation and a storage destination, but at least
one must be provided.

All event transport over a **Subscription** is 1:1 – only a single `from`,
`call`, and `result` may be provided.

For more details, see [Kind: Subscription](spec.md#kind-subscription).

## Source

**Source** represents an ongoing selection of events from an external system,
such as object creation events in a specific storage bucket, database updates
in a particular table, or Kubernetes resource state changes. Because a
**Source** represents an external system, it only produces events (and is
therefore _Subscribable_ by **Subscriptions**). **Source** configures an
external system and may include parameters such as specific resource names,
event types, or credentials which should be used to establish the connection to
the external system. The set of allowed configuration parameters is described
by the **Provisioner** which is referenced by the **Source**.

Event selection on a **Source** is 1:N – a single **Source** may fan out to
multiple **Subscriptions**.

For more details, see [Kind: Source](spec.md#kind-source).

## Channel

**Channel** provides a durable event storage mechanism which can fan out
received events to multiple destinations via **Subscriptions**. A **Channel**
has a single inbound _Sinkable_ interface which may accept events from multiple
**Subscriptions** or even direct delivery from external systems. Different
**Channels** may implement different degrees of persistence.

See [Kind: Channel](spec.md#kind-channel).

## Provisioner

The eventing API has parallel constructs for event _sources_ (systems which
create events based on internal or external changes) and event _transports_
(middleware which add value to the event delivery chain, such as persistence or
buffering).

For more details, see [Kind: Provider](spec.md#kind-provisioner).

--- 

*Next*:

* [Motivation and goals](motivation.md)
<!-- * [Resource type overview](overview.md) -->
* [Interface contracts](interfaces.md)
* [Object model specification](spec.md)

