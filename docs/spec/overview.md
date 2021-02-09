# Resource Types

The API defines and provides a complete implementation for
[Trigger](spec.md#kind-trigger), [Broker](spec.md#kind-broker),
[Subscription](spec.md#kind-subscription) and abstract resource definitions for
[Channels](spec.md#kind-channel).

With extensibility and composability as a goal of Knative Eventing, the eventing
API defines several resources that can be reduced down to well understood
contracts. These eventing resource interfaces may be fulfilled by other
Kubernetes objects and then composed in the same way as the concrete objects.
The interfaces are ([Addressable](interfaces.md#addressable),
[Callable](interfaces.md#callable)). For more details, see
[Interface Contracts](interfaces.md).

- A **Trigger** describes a filter on event attributes which should be delivered
  to an _Addressable_.

- A **Broker** provides a bucket of events which can be selected by attribute.

<!-- https://drive.google.com/open?id=1CXRvT2g6sxk6-ZrwYcSf2BahCNVlLTLNZkm-laQitMg -->

![Broker Trigger Overview](images/broker-trigger-overview.svg)

The above diagram shows a _Broker_ ingesting events and delivering to a
_Service_ only when the _Trigger_ filter matches.

- A **Subscription** describes the transformation of an event and optional
  forwarding of a returned event.

- A **Channel** provides event persistence and fanout of events from a
  well-known input address to multiple outputs described by _Subscriptions_.

<!-- This image is sourced from https://drive.google.com/open?id=10mmXzDb8S_4_ZG_hcBr7s4HPISyBqcqeJLTXLwkilRc -->

![Resource Types Overview](images/resource-types-overview.svg)

Channels as well as Sources are defined by independent CRDs that can be
installed into a cluster. Both Sources and Channels implementations can be created directly.
A Channel however offers also a way to create the backing implementation as part of the
generic Channel (using a _channelTemplate_).

## Trigger

**Trigger** describes a registration of interest on a filter set of events
delivered to a _Broker_ which should be delivered to an _Addressable_. Events
selected by a _Trigger_ are buffered independently from other _Triggers_, even
if they deliver to the same _Addressable_.

For more details, see [Kind: Trigger](spec.md#kind-trigger).

## Broker

**Broker** provides an eventing mesh. This allows producers to deliver events to
a single endpoint and not need to worry about the routing details for individual
consumers.

For more details, see [Kind: Broker](spec.md#kind-broker).

## Subscription

**Subscriptions** describe a flow of events from one _Channel_ to the next
Channel\* through transformations (such as a Knative Service which processes
CloudEvents over HTTP). A _Subscription_ controller resolves the addresses of
transformations (`subscriber`) and destination storage (`reply`) through the
_Callable_ and _Addressable_ interface contracts, and writes the resolved
addresses to the _Channel_ in the `channel` reference. _Subscriptions_ do not
need to specify both a transformation and a storage destination, but at least
one must be provided.

All event delivery linkage from a **Subscription** is 1:1 – only a single
`channel`, `subscriber`, and `reply` may be provided.

For more details, see [Kind: Subscription](spec.md#kind-subscription).

## Channel

**Channel** provides an event delivery mechanism which can fan out received
events to multiple destinations via _Subscriptions_. A _Channel_ has a single
inbound _Addressable_ interface which may accept events delivered directly or
forwarded from multiple _Subscriptions_. Different _Channels_ may implement
different degrees of persistence. Event delivery order is dependent on the
backing implementation of the _Channel_.

Event selection on a _Channel_ is 1:N – a single _Channel_ may fan out to
multiple _Subscriptions_.

See [Kind: Channel](spec.md#kind-channel).

---

_Navigation_:

- [Motivation and goals](motivation.md)
- **Resource type overview**
- [Interface contracts](interfaces.md)
- [Object model specification](spec.md)
