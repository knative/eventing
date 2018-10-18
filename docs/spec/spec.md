# Object Model

Knative ships with several objects which implement useful sets of
[interfaces](interfaces.md). It is expected that additional objects which
implement other sets of interfaces will be added over time. For context, see
the [overview](overview.md) and [motivations](motivation.md) sections.

These are Kubernetes resources that been introduced using Custom Resource
Definitions. They will have the expected _ObjectMeta_, _Spec_, _Status_ fields.
This document details our _Spec_ and _Status_ customizations.

- [Source](#kind-source)
- [Channel](#kind-channel)
- [Subscription](#kind-subscription)
- [Provider](#kind-provisioner)

---

## kind: Source

### group: eventing.knative.dev/v1alpha1

_Describes a specific configuration (credentials, etc) of a source system which
can be used to supply events. A common pattern is for Sources to emit events to
Channel to allow event delivery to be fanned-out within the cluster. They
cannot receive events._

### Object Schema

#### Spec

| Field         | Type                               | Description                                                                                                            | Limitations                                               |
| ------------- | ---------------------------------- | ---------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| provisioner\* | ProvisionerReference               | The provisioner used to create any backing resources and configuration.                                                | Immutable.                                                |
| arguments     | runtime.RawExtension (JSON object) | Arguments passed to the provisioner for this specific source.                                                          | Arguments must validate against provisioner's parameters. |
| channel       | ObjectRef                          | Specify an existing channel to use to emit events. If empty, create a new Channel using the cluster/namespace default. | Source will not emit events until channel exists.         |

\*: Required

#### Status

| Field        | Type                      | Description                                                                                  | Limitations                                              |
| ------------ | ------------------------- | -------------------------------------------------------------------------------------------- | -------------------------------------------------------- |
| subscribable | Subscribable              | Pointer to a channel which can be subscribed to in order to receive events from this source. |                                                          |
| provisioned  | []ProvisionedObjectStatus | Creation status of each Channel and errors therein.                                          | It is expected that a Source list all produced Channels. |
| conditions   | Conditions                | Source conditions.                                                                           |                                                          |

##### Conditions

- **Ready.** True when the Source is provisioned and ready to emit events.
- **Provisioned.** True when the Source has been provisioned by a controller.

#### Events

- Provisioned - describes each resource that is provisioned.

### Life Cycle

| Action | Reactions                                                                                                 | Limitations |
| ------ | --------------------------------------------------------------------------------------------------------- | ----------- |
| Create | Provisioner controller watches for Sources and creates the backing resources depending on implementation. |             |
| Update | Provisioner controller synchronizes backing implementation on changes.                                    |             |
| Delete | Provisioner controller will deprovision backing resources depending on implementation.                    |             |

---

## kind: Channel

### group: eventing.knative.dev/v1alpha1

_A Channel logically receives events on its input domain and forwards them to
its subscribers. Additional behavior may be introduced by using the
Subscription's call parameter._

### Object Schema

#### Spec

| Field         | Type                               | Description                                                                | Limitations                          |
| ------------- | ---------------------------------- | -------------------------------------------------------------------------- | ------------------------------------ |
| provisioner\* | ProvisionerReference               | The name of the provisioner to create the resources that back the Channel. | Immutable.                           |
| arguments     | runtime.RawExtension (JSON object) | Arguments to be passed to the provisioner.                                 |                                      |
| channelable   | Channelable                        | Holds a list of downstream subscribers for the channel.                    |                                      |

\*: Required

#### Metadata

##### Owner References

- If the Source controller created this Channel: Owned by the originating
  Source.
- Owned (non-controlling) by the Provisioner used to provision the Channel.

#### Status

| Field        | Type         | Description                                                                                                                 | Limitations |
| ------------ | ------------ | --------------------------------------------------------------------------------------------------------------------------- | ----------- |
| sinkable     | Sinkable     | Address to the endpoint as top-level domain that will distribute traffic over the provided targets from inside the cluster. |             |
| subscribable | Subscribable |                                                                                                                             |             |
| conditions   | Conditions   | Standard Subscriptions                                                                                                      |             |

##### Conditions

- **Ready.** True when the Channel is provisioned and ready to accept events.
- **Provisioned.** True when the Channel has been provisioned by a controller.

#### Events

- Provisioned
- Deprovisioned

### Life Cycle

| Action | Reactions                                                                                                                                                        | Limitations                                                          |
| ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| Create | The Provisioner referenced will take ownership of the Channel and begin provisioning the backing resources required for the Channel depending on implementation. | Only one Provisioner is allowed to be the Owner for a given Channel. |
| Update | The Provisioner will synchronize the Channel backing resources to reflect the update.                                                                            |                                                                      |
| Delete | The Provisioner will deprovision the backing resources if no longer required depending on implementation.                                                        |                                                                      |

---

## kind: Subscription

### group: eventing.knative.dev/v1alpha1

_Describes a linkage between a Subscribable and a Targetable and/or Sinkable._

### Object Schema

#### Spec

| Field              | Type           | Description                                                                  | Limitations        |
| ------------------ | -------------- | ---------------------------------------------------------------------------- | ------------------ |
| from\*             | ObjectRef      | The originating Channel for the link.                                        | Must be a Channel. |
| call<sup>1</sup>   | EndpointSpec   | Optional processing on the event. The result of call will be sent to result. |                    |
| result<sup>1</sup> | ResultStrategy | The continuation for the link.                                               |                    |

\*: Required

1: At Least One(call, result)

#### Metadata

##### Owner References

- If a resource controller created this Subscription: Owned by the originating
  resource.

##### Conditions

- **Ready.**
- **FromReady.**
- **CallActive.** True if the call is sinking events without error.
- **Resolved.**

#### Events

- PublisherAcknowledged
- ActionFailed

### Life Cycle

| Action | Reactions                                                        | Limitations |
| ------ | ---------------------------------------------------------------- | ----------- |
| Create | The publisher referenced needs to be watching for Subscriptions. |             |
| Update |                                                                  |             |
| Delete |                                                                  |             |

---

## kind: Provisioner

### group: eventing.knative.dev/v1alpha1

_Describes an abstract configuration of a Source system which produces events
or a Channel system that receives and delivers events._

### Object Schema

#### Spec

| Field  | Type                                                                            | Description                                 | Limitations                |
| ------ | ------------------------------------------------------------------------------- | ------------------------------------------- | -------------------------- |
| type\* | [GroupKind](https://godoc.org/k8s.io/apimachinery/pkg/runtime/schema#GroupKind) | The type of the resource to be provisioned. | Must be Source or Channel. |

\*: Required

#### Status

| Field       | Type                      | Description            | Limitations |
| ----------- | ------------------------- | ---------------------- | ----------- |
| conditions  | Conditions                | Provisioner conditions |             |

##### Conditions

- **Ready.**

#### Events

- Source created
- Source deleted

---

## Shared Object Schema

### ProvisionerReference

| Field | Type            | Description | Limitations |
| ----- | --------------- | ----------- | ----------- |
| ref\* | ObjectReference |             |             |

\*: Required

### EndpointSpec

| Field                 | Type            | Description | Limitations                |
| --------------------- | --------------- | ----------- | -------------------------- |
| targetRef<sup>1</sup> | ObjectReference |             | Must adhere to Targetable. |
| dnsName<sup>1</sup>   | String          |             |                            |

1: One of (targetRef, dnsName), Required.

### ProvisionedObjectStatus

| Field    | Type   | Description                                                  | Limitations |
| -------- | ------ | ------------------------------------------------------------ | ----------- |
| name\*   | String | Name of Object                                               |             |
| type\*   | String | Fully Qualified Object type.                                 |             |
| status\* | String | Current relationship between Provisioner and Object.         |             |
| reason   | String | Detailed description describing current relationship status. |             |

\*: Required

### Subscribable

| Field       | Type            | Description                      | Limitations |
| ----------- | --------------- | -------------------------------- | ----------- |
| channelable | ObjectReference | The channel used to emit events. |             |

### Channelable

| Field       | Type                    | Description                                                           | Limitations                            |
| ----------- | ----------------------- | --------------------------------------------------------------------- | -------------------------------------- |
| subscribers | ChannelSubscriberSpec[] | Information about subscriptions used to implement message forwarding. | Filled out by Subscription Controller. |

### ChannelSubscriberSpec

| Field          | Type   | Description                                     | Limitations |
| -------------- | ------ | ----------------------------------------------- | ----------- |
| callableDomain | String | The domain name of the endpoint for the call.   |             |
| sinkableDomain | String | The domain name of the endpoint for the result. |             |

### ResultStrategy

| Field    | Type      | Description                            | Limitations        |
| -------- | --------- | -------------------------------------- | ------------------ |
| target\* | ObjectRef | The continuation Channel for the link. | Must be a Channel. |

\*: Required
