# Object Model

Knative ships with several objects which implement useful sets of
[interfaces](interfaces.md). It is expected that additional objects which
implement these interfaces will be added over time. For context, see the
[overview](overview.md) and [motivations](motivation.md) sections.

These are Kubernetes resources that been introduced using Custom Resource
Definitions. They will have the expected _ObjectMeta_, _Spec_, _Status_ fields.
This document details our _Spec_ and _Status_ customizations.

- [Channel](#kind-channel)
- [Subscription](#kind-subscription)
- [ClusterChannelProvisioner](#kind-clusterchannelprovisioner)

## kind: Channel

### group: eventing.knative.dev/v1alpha1

_A Channel logically receives events on its input domain and forwards them to
its subscribers._

### Object Schema

#### Spec

| Field                    | Type                               | Description                                                                | Constraints                            |
| ------------------------ | ---------------------------------- | -------------------------------------------------------------------------- | -------------------------------------- |
| provisioner\*            | ObjectReference                    | The name of the provisioner to create the resources that back the Channel. | Immutable.                             |
| arguments                | runtime.RawExtension (JSON object) | Arguments to be passed to the provisioner.                                 |                                        |
| subscribable.subscribers | ChannelSubscriberSpec[]            | Information about subscriptions used to implement message forwarding.      | Filled out by Subscription Controller. |

\*: Required

#### Metadata

##### Owner References

- Owned (non-controlling) by the ClusterChannelProvisioner used to provision
  the Channel.

#### Status

| Field      | Type        | Description                                                                                  | Constraints |
| ---------- | ----------- | -------------------------------------------------------------------------------------------- | ----------- |
| address    | Addressable | Address of the endpoint which meets the [_Addressable_ contract](interfaces.md#addressable). |             |
| conditions | Conditions  | Channel conditions.                                                                          |             |

##### Conditions

- **Ready.** True when the Channel is provisioned and ready to accept events.
- **Provisioned.** True when the Channel has been provisioned by a controller.

#### Events

- Provisioned
- Deprovisioned

### Life Cycle

| Action | Reactions                                                                                                                                                                      | Constraints                                                                        |
| ------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------------------------------------------------------------------------------- |
| Create | The ClusterChannelProvisioner referenced will take ownership of the Channel and begin provisioning the backing resources required for the Channel depending on implementation. | Only one ClusterChannelProvisioner is allowed to be the Owner for a given Channel. |
| Update | The ClusterChannelProvisioner will synchronize the Channel backing resources to reflect the update.                                                                            |                                                                                    |
| Delete | The ClusterChannelProvisioner will deprovision the backing resources if no longer required depending on implementation.                                                        |                                                                                    |

---

## kind: Subscription

### group: eventing.knative.dev/v1alpha1

_Describes a linkage between a Channel and a Callable and/or Addressable channel._

### Object Schema

#### Spec

| Field                  | Type           | Description                                                                       | Constraints        |
| ---------------------- | -------------- | --------------------------------------------------------------------------------- | ------------------ |
| channel\*              | ObjectRef      | The originating _Subscribable_ for the link.                                      | Must be a Channel. |
| subscriber<sup>1</sup> | SubscriberSpec | Optional processing on the event. The result of subscriber will be sent to reply. |                    |
| reply<sup>1</sup>      | ReplyStrategy  | The continuation for the link.                                                    |                    |

\*: Required

1: At Least One(subscriber, reply)

#### Metadata

##### Owner References

- If a resource controller created this Subscription: Owned by the originating
  resource.

##### Conditions

- **Ready.**
- **FromReady.**
- **Resolved.** True if `channel`, `subscriber`, and `reply` all resolve into valid object references which implement the appropriate spec.

#### Events

- PublisherAcknowledged
- ActionFailed

### Life Cycle

| Action | Reactions                                                                                                                                           | Constraints |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| Create | The subscription controller adds the resolved URIs of `subscriber` and `reply` to the `subscribers` field in the `channel` _Subscribable_ resource. |             |
| Update |                                                                                                                                                     |             |
| Delete |                                                                                                                                                     |             |

---

## kind: ClusterChannelProvisioner

### group: eventing.knative.dev/v1alpha1

_Describes an abstract configuration of a Source system which produces events
or a Channel system that receives and delivers events._

### Object Schema

#### Spec

| Field      | Type                               | Description                                                                                       | Constraints |
| ---------- | ---------------------------------- | ------------------------------------------------------------------------------------------------- | ----------- |
| parameters | runtime.RawExtension (JSON object) | Description of the arguments able to be passed by the provisioned resource (not enforced in 0.1). | JSON Schema |

\*: Required

#### Status

| Field      | Type       | Description                          | Constraints |
| ---------- | ---------- | ------------------------------------ | ----------- |
| conditions | Conditions | ClusterChannelProvisioner conditions |             |

##### Conditions

- **Ready.**

#### Events

- Resource Created.
- Resource Removed.

---

## Shared Object Schema

### SubscriberSpec

| Field               | Type            | Description | Constraints              |
| ------------------- | --------------- | ----------- | ------------------------ |
| ref<sup>1</sup>     | ObjectReference |             | Must adhere to Callable. |
| dnsName<sup>1</sup> | String          |             |                          |

1: One of (ref, dnsName), Required.

### ChannelSubscriberSpec

| Field         | Type            | Description                                                    | Constraints    |
| ------------- | --------------- | -------------------------------------------------------------- | -------------- |
| ref           | ObjectReference | The Subscription this ChannelSubscriberSpec was resolved from. |                |
| subscriberURI | String          | The URI name of the endpoint for the subscriber.               | Must be a URL. |
| replyURI      | String          | The URI name of the endpoint for the reply.                    | Must be a URL. |

### ReplyStrategy

| Field     | Type      | Description                            | Constraints        |
| --------- | --------- | -------------------------------------- | ------------------ |
| channel\* | ObjectRef | The continuation Channel for the link. | Must be a Channel. |

\*: Required
