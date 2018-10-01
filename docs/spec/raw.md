<!-- TODO: this is the heavily modified output of https://docs.google.com/document/d/1JZtFkz_C3orG9IfDxxGkAmuuGxUQ434ji-rdl3oKObE/edit -->

# Eventing Specification - v1alpha1

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
be fulfilled by other Kubernetes objects ([Callable](#callable),
[Subscribable](#subscribable), [Channelable](#channelable),
[Targetable](#targetable)). The API defines and provides a complete
implementation for [Subscription](#kind-subscription), and abstract resource
definitions for [Sources](#kind-source), [Channels](#kind-channel), and
[Providers](#kind-provisioner) which may be fulfilled by multiple backing
implementations (much like the Kubernetes Ingress resource).

 * A **Subscription** describes the transformation of an event (via a
   _Callable_) and optional storage of a returned event.

 * A **Source** represents an ongoing selection of events from an external
   system. A Flow is used to connect these events to subsequent processing
   steps.

 * A **Channel** provides event storage and fanout of events from a well-known
   input address to multiple outputs described by Flows.

<!-- TODO(): insert the source subscriptions -->

 * **Provisioners** act as a catalog of the types of Sources and Channels
   currently active in the eventing system.

<!-- TODO(): insert the source and channel -->

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

## Channel

**Channel** provides a durable event storage mechanism which can fan out
received events to multiple destinations via **Subscriptions**. A **Channel**
has a single inbound _Sinkable_ interface which may accept events from multiple
**Subscriptions** or even direct delivery from external systems. Different
**Channels** may implement different degrees of persistence,

## Provisioner

The eventing API has parallel constructs for event _sources_ (systems which
create events based on internal or external changes) and event _transports_
(middleware which add value to the event delivery chain, such as persistence or
buffering).


# Interface Contracts

<!-- TODO: Insert drawing of interfaces. -->

## Subscribable

A **Subscribable** resource will emit events that a _Subscription_ can direct
to a _Targetable_ or _Sinkable_ resource.

### Control Plane

A **Subscribable** resource may be referenced in the _from_ section of a
_Flow_. The **Subscribable** resource MUST expose a _channel_ field (an
ObjectReference) in its _status_ section. This _channel_ field MUST be
_Channelable_, and _channel_ MAY refer back to the **Subscribable** resource.

### Data Plane

A **Subscribable** resource is an event producer or forwarder, events that it
produces or forwards are delivered via its _status.channel_ resource.

## Channelable

A **Channelable** resource owns a list of subscribers for delivery of events.

### Control Plane

The **Channelable** resource has a list of _subscribers_ within the resources
_spec_. In practice, the resolved _subscription_ _call_ and _result_ endpoints
populate the Channelable's list of _subscribers_.

### Data Plane

**Channelable** resources will attempt delivery to each of they _subscribers_
at least once, and retry if the callie returns errors.

## Targetable

A **Targetable** resource represents a unit of work to be done on/to/with an
event. They allow for event modification and flow control without taking
ownership of the event. Typically a **Targetable** is a function.

### Control Plane

A **Targetable** resource MUST expose a _targetable.domainInternal_ field in
its _status_ section. The _domainInternal_ value is an internal domain name
that is capable of receiving event deliveries. **Targetable** resources may be
referenced in the _call_ section of a _Subscription_.

### Data Plane

The **Targetable** resource does not ownership of the event that was passed to
it, but it is allowed to modify the event by returning a response code of 200
and a mutated version back to the caller. The **Targetable** can prevent the
event from continuing to the next stage by returning 200 and an empty body. All
other response codes are considered an error and the caller will attempt to
call again.

## Sinkable

A **Sinkable** resource takes ownership of an event. Typically a **Sinkable**
is a Channel.

### Control Plane

A **Sinkable** resource MUST expose a _targetable.domainInternal_ field in its
_status_ section. The _domainInternal_ value is an internal domain name that is
capable of receiving event deliveries. **Sinkable** resources** **may be
referenced in the _result_ section of a _Subscription_.

### Data Plane

A **Sinkable** resource will only respond to requests by ACK/NACK. It must not
return events to the caller.


# kind: Source

## group: eventing.knative.dev/v1alpha1

_Describes a specific configuration (credentials, etc) of a source system which
can be used to supply events. Sources emit events using a channel specified in
their status. They cannot receive events._

## Object Schema

### Spec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| provisioner*| ProvisionerReference | The provisioner used to create any backing resources and configuration. | Immutable. |
| arguments | runtime.RawExtension (JSON object)| Arguments passed to the provisioner for this specific source. | Arguments must validate against provisioner's parameters. |
| channel | ObjectRef | Specify an existing channel to use to emit events. If empty, create a new Channel using the cluster/namespace default. | Source will not emit events until channel exists. |

\*: Required

### Metadata

#### Owner References

 * Owned by a Flow if created by a Flow.

### Status

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| channel | ObjectReference | The channel used to emit events. | |
| Subscribable | Subscribable | Pointer to a channel which can be subscribed to in order to receive events from this source. | |
| provisioned |[]ProvisionedObjectStatus| Creation status of each Channel and errors therein. | It is expected that a Source list all produced Channels. |

#### Conditions

 * **Ready.** True when the Source is provisioned and ready to emit events.
 * ** Provisioned.** True when the Source has been provisioned by a controller.

### Events

 * Provisioned - describes each resource that is provisioned.

## Life Cycle

| Action | Reactions | Limitations |
| --- | --- | --- |
| Create | Provisioner controller watches for Sources and creates the backing resources depending on implementation. | |
| Update | Provisioner controller synchronizes backing implementation on changes. | |
| Delete | Provisioner controller will deprovision backing resources depending on implementation. | Flow controller will recreate a Source after deletion if the Source originated from a Flow. |

# kind: Channel

## group: eventing.knative.dev/v1alpha1

_A Channel logically receives events on its input domain and forwards them to
its subscribers. Additional behavior may be introduced by using the
Subscription's call parameter._

## Object Schema

### Spec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| provisioner*| ProvisionerReference | The name of the provisioner to create the resources that back the Channel. | Immutable. |
| arguments | runtime.RawExtension (JSON object)| Arguments to be passed to the provisioner. | |
| channelable | Channelable | Holds a list of downstream subscribers for the channel. | |
| eventTypes |[]String| An array of event types that will be passed on the Channel. | Must be objects with kind:EventType. |

\*: Required

### Metadata

#### Owner References

 * If the Pipeline controller created this Channel: Owned by the originating Pipeline.
 * Owned by the Provisioner used to provision the Channel.

### Status

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| Sinkable | Sinkable | Address to the endpoint as top-level domain that will distribute traffic over the provided targets from inside the cluster. | |
| Subscribable | Subscribable | | |
| Conditions | Conditions | Standard Subscriptions| |

#### Conditions

 * **Ready.** True when the Channel is provisioned and ready to accept events.
 * **Provisioned.** True when the Channel has been provisioned by a controller.

### Events

 * Provisioned
 * Deprovisioned

## Life Cycle

| Action | Reactions | Limitations |
| --- | --- | --- |
| Create | The Provisioner referenced will take ownership of the Channel and begin provisioning the backing resources required for the Channel depending on implementation. | Only one Provisioner is allowed to be the Owner for a given Channel. |
| Update | The Provisioner will synchronize the Channel backing resources to reflect the update. | |
| Delete | The Provisioner will deprovision the backing resources if no longer required depending on implementation. | |

# kind: Provisioner

## group: eventing.knative.dev/v1alpha1

_Describes an abstract configuration of a Source system which produces events
or a Channel system that receives and delivers events._

## Object Schema

### Spec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| type*| TypeMeta<sup>1</sup> | The type of the resource to be provisioned. | Must be Source or Channel. |
| parameters |[]ParameterSpec| Parameters are used for validation of arguments. | |

\*: Required

1: Kubernetes type.

### Metadata

#### Owner References

 * Owns EventTypes.

### Status

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| provisioned |[]ProvisionedObjectStatus| Status of creation or adoption of each EventType and errors therein. | It is expected that a provisioner list all produced EventTypes, if applicable. |

### Events

 * Source created
 * Source deleted
 * Event types installed

## Life Cycle

| Action | Reactions | Limitations |
| --- | --- | --- |
| Create | Creates and owns EventTypes produced, or adds Owner ref to existing EventTypes. | Verifies Json Schema provided by existing EventTypes; Not allowed to edit EventType if previously Owned; |
| Update | Synchronizes EventTypes. | |
| Delete | Removes Owner ref from EventTypes. | |

# kind: Subscription

## group: eventing.knative.dev/v1alpha1

_Describes a direct linkage between an event publisher and an action._

## Object Schema

### Spec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| from*| ObjectRef | The originating Channel for the link. | Must be a Channel. |
| call<sup>1</sup> | EndpointSpec | Optional processing on the event. The result of call will be sent to result. | |
| result<sup>1</sup> | ObjectRef | The continuation Channel for the link. | Must be a Channel. |

\*: Required

1: At Least One(call, result)

### Metadata

#### Owner References

 * If a resource controller created this Subscription: Owned by the originating resource.

### Status

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| resolutions | SubscriptionResolutionsStatus | Resolved targets for the spec's call and continue fields. | |

#### Conditions

 * **Ready.** True ~~when the publisher resource is also ready.
 * **FromReady.**
 * **CallActive. **True if the call is sinking events without error.
 * **Resolved**

### Events

 * PublisherAcknowledged
 * ActionFailed

## Life Cycle

| Action | Reactions | Limitations |
| --- | --- | --- |
| Create | The publisher referenced needs to be watching for Subscriptions. | |
| Update | | |
| Delete | | |

# Shared Object Schema

## ProvisionerReference

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| ref<sup>1</sup> | ObjectReference | | |
| selector<sup>1</sup> | LabelSelector | | |

1: One of (name, selector), Required.

## EndpointSpec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| targetRef<sup>1</sup> | ObjectReference | | Must adhere to Targetable. |
| dnsName<sup>1</sup> | String | | |

1: One of (targetRef, dnsName), Required.

## ParameterSpec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| name*| String | The unique name of this template parameter. | |
| description | String | A human-readable explanation of this template parameter. | |
| required | Boolean | If the parameter is required. | If true, default / defaultFrom should not be provided. Immutable. |
| default<sup>1</sup> | String | Default value if not provided by arguments. | |
| defaultFrom<sup>1</sup> | ArgumentValueReference | Default value retrieved from an existing Secret or ConfigMap if not provided by arguments. | |

\*: Required

1: OneOf (default, defaultFrom)

## ArgumentValueReference

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| secretKeyRef<sup>1</sup> | SecretReference | A reference to a value contained in a Secret at the given key. | |
| configMapRef<sup>1</sup> | ConfigMapReference | A reference to a value contained in a ConfigMap at the given key. | |

1: OneOf (secretKeyRef, configMapRef), Required.

## ProvisionedObjectStatus

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| name*| String | Name of Object| |
| type*| String | Fully Qualified Object type. | |
| status*| String | Current relationship between Provisioner and Object| |
| reason | String | Detailed description describing current relationship status. | |

\*: Required

## SubscriptionResolutionsStatus

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| call | String | Resolved target for the spec's call field. Empty string if spec.call is nil. | Must be a domain name |
| continue | String | Resolved target for the spec's continue field. Empty string if spec.continue is nil. | Must be a domain name |

## Subscribable

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| channelable | ObjectReference | The channel used to emit events. | |

## Channelable

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| subscribers | ChannelSubscriberSpec[]| Information about subscriptions used to implement message forwarding. | Filled out by Subscription Controller. |

## ChannelSubscriberSpec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| callableDomain | String | The domain name of the endpoint for the call. | |
| sinkableDomain | String | The domain name of the endpoint for the result. | |
