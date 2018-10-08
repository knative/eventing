# Object Model

This is the detailed specification of the Knative Eventing Object Model. For
context, see the [overview](overview.md), [motivation](motivation.md), and the
[interface contract](interfaces.md) documents.

 * [Source](#kind-source) 
 * [Channel](#kind-channel)
 * [Subscription](#kind-subscription)
 * [Provider](#kind-provisioner)

---

## kind: Source

### group: eventing.knative.dev/v1alpha1

_Describes a specific configuration (credentials, etc) of a source system which
can be used to supply events. Sources emit events using a channel specified in
their status. They cannot receive events._

### Object Schema

#### Spec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| provisioner*| ProvisionerReference | The provisioner used to create any backing resources and configuration. | Immutable. |
| arguments | runtime.RawExtension (JSON object)| Arguments passed to the provisioner for this specific source. | Arguments must validate against provisioner's parameters. |
| channel | ObjectRef | Specify an existing channel to use to emit events. If empty, create a new Channel using the cluster/namespace default. | Source will not emit events until channel exists. |

\*: Required

#### Status

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| subscribable | Subscribable | Pointer to a channel which can be subscribed to in order to receive events from this source. | |
| provisioned |[]ProvisionedObjectStatus| Creation status of each Channel and errors therein. | It is expected that a Source list all produced Channels. |

##### Conditions

 * **Ready.** True when the Source is provisioned and ready to emit events.
 * **Provisioned.** True when the Source has been provisioned by a controller.

#### Events

 * Provisioned - describes each resource that is provisioned.

### Life Cycle

| Action | Reactions | Limitations |
| --- | --- | --- |
| Create | Provisioner controller watches for Sources and creates the backing resources depending on implementation. | |
| Update | Provisioner controller synchronizes backing implementation on changes. | |
| Delete | Provisioner controller will deprovision backing resources depending on implementation. | |

---

## kind: Channel

### group: eventing.knative.dev/v1alpha1

_A Channel logically receives events on its input domain and forwards them to
its subscribers. Additional behavior may be introduced by using the
Subscription's call parameter._

### Object Schema

#### Spec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| provisioner*| ProvisionerReference | The name of the provisioner to create the resources that back the Channel. | Immutable. |
| arguments | runtime.RawExtension (JSON object)| Arguments to be passed to the provisioner. | |
| channelable | Channelable | Holds a list of downstream subscribers for the channel. | |
| eventTypes |[]String| An array of event types that will be passed on the Channel. | Must be objects with kind:EventType. |

\*: Required

#### Metadata

##### Owner References

 * If the Source controller created this Channel: Owned by the originating
   Source.
 * Owned (non-controlling) by the Provisioner used to provision the Channel.

#### Status

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| Sinkable | Sinkable | Address to the endpoint as top-level domain that will distribute traffic over the provided targets from inside the cluster. | |
| subscribable | Subscribable | | |
| Conditions | Conditions | Standard Subscriptions| |

##### Conditions

 * **Ready.** True when the Channel is provisioned and ready to accept events.
 * **Provisioned.** True when the Channel has been provisioned by a controller.

#### Events

 * Provisioned
 * Deprovisioned

### Life Cycle

| Action | Reactions | Limitations |
| --- | --- | --- |
| Create | The Provisioner referenced will take ownership of the Channel and begin provisioning the backing resources required for the Channel depending on implementation. | Only one Provisioner is allowed to be the Owner for a given Channel. |
| Update | The Provisioner will synchronize the Channel backing resources to reflect the update. | |
| Delete | The Provisioner will deprovision the backing resources if no longer required depending on implementation. | |

---

## kind: Provisioner

### group: eventing.knative.dev/v1alpha1

_Describes an abstract configuration of a Source system which produces events
or a Channel system that receives and delivers events._

### Object Schema

#### Spec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| type*| GroupKind<sup>1</sup> | The type of the resource to be provisioned. | Must be Source or Channel. |

\*: Required

1: Kubernetes type.

#### Metadata

##### Owner References

 * Owns EventTypes.

#### Status

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| provisioned |[]ProvisionedObjectStatus| Status of creation or adoption of each EventType and errors therein. | It is expected that a provisioner list all produced EventTypes, if applicable. |

#### Events

 * Source created
 * Source deleted
 * Event types installed

### Life Cycle

| Action | Reactions | Limitations |
| --- | --- | --- |
| Create | Creates and owns EventTypes produced, or adds Owner ref to existing EventTypes. | Verifies Json Schema provided by existing EventTypes; Not allowed to edit EventType if previously Owned; |
| Update | Synchronizes EventTypes. | |
| Delete | Removes Owner ref from EventTypes. | |

---

## kind: Subscription

### group: eventing.knative.dev/v1alpha1

_Describes a linkage between a Subscribable and a Targetable and/or Sinkable._

### Object Schema

#### Spec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| from*| ObjectRef | The originating Channel for the link. | Must be a Channel. |
| call<sup>1</sup> | EndpointSpec | Optional processing on the event. The result of call will be sent to result. | |
| result<sup>1</sup> | ObjectRef | The continuation Channel for the link. | Must be a Channel. |

\*: Required

1: At Least One(call, result)

#### Metadata

##### Owner References

 * If a resource controller created this Subscription: Owned by the originating resource.

##### Conditions

 * **Ready.**
 * **FromReady.**
 * **CallActive.** True if the call is sinking events without error.
 * **Resolved.**

#### Events

 * PublisherAcknowledged
 * ActionFailed

### Life Cycle

| Action | Reactions | Limitations |
| --- | --- | --- |
| Create | The publisher referenced needs to be watching for Subscriptions. | |
| Update | | |
| Delete | | |

---


## kind: EventType

NOTE: EventType is out of scope for 0.1 release. This is future documentation.

### group: eventing.knative.dev/v1alpha1

_Describes a particular schema of Events which may be produced by one or more
source systems._

### Object Schema

#### Spec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| jsonSchema | String | The Json Schema that represents an event in flight. | Only for JSON transport types. |

#### Metadata

##### Owner References

EventType is owned by *Provisioners*. Each *Provisioner* creates a
non-controlling OwnerReference on the EventType resources it knows about.

#### Status

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| referenceCount | Integer | Number of Owners for this EventType | |

##### Conditions

 * **Pinned**. True when the EventType cannot be deleted.

#### Events

 * **Owned.** When EventType has a new Provisioner Owner.
 * **Released.** When a Provisioner removes Ownership from the EventType.

### Life Cycle

| Action | Reactions | Limitations |
| --- | --- | --- |
| Create | | |
| Update | | |
| Delete | | Blocked until all provisioners release it. |

---

## Shared Object Schema

### ProvisionerReference

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| ref<sup>1</sup> | ObjectReference | | |
| selector<sup>1</sup> | LabelSelector | | Must match only one resource. |

1: One of (name, selector), Required.

### EndpointSpec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| targetRef<sup>1</sup> | ObjectReference | | Must adhere to Targetable. |
| dnsName<sup>1</sup> | String | | |

1: One of (targetRef, dnsName), Required.

### ProvisionedObjectStatus

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| name*| String | Name of Object| |
| type*| String | Fully Qualified Object type. | |
| status*| String | Current relationship between Provisioner and Object. | |
| reason | String | Detailed description describing current relationship status. | |

\*: Required

### Subscribable

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| channelable | ObjectReference | The channel used to emit events. | |

### Channelable

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| subscribers | ChannelSubscriberSpec[]| Information about subscriptions used to implement message forwarding. | Filled out by Subscription Controller. |

### ChannelSubscriberSpec

| Field | Type | Description | Limitations |
| --- | --- | --- | --- |
| callableDomain | String | The domain name of the endpoint for the call. | |
| sinkableDomain | String | The domain name of the endpoint for the result. | |
