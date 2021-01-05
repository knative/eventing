# Object Model

Knative ships with several objects which implement useful sets of
[interfaces](interfaces.md). It is expected that additional objects which
implement these interfaces will be added over time. For context, see the
[overview](overview.md) and [motivations](motivation.md) sections.

These are Kubernetes resources that been introduced using Custom Resource
Definitions. They will have the expected _ObjectMeta_, _Spec_, _Status_ fields.
This document details our _Spec_ and _Status_ customizations.

- [Trigger](#kind-trigger)
- [Broker](#kind-broker)
- [Channel](#kind-channel)
- [Subscription](#kind-subscription)

## kind: Trigger

### group: eventing.knative.dev/v1beta1

_A Trigger represents a subscriber of events with a filter for a specific
broker._

### Object Schema

#### Spec

| Field        | Type                 | Description                                                                                                                                                                         | Constraints |
| ------------ | -------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| broker       | String               | Broker is the broker that this trigger receives events from. Defaults to 'default'.                                                                                                 |             |
| filter       | TriggerFilter        | Optional Filter is the filter to apply against all events from the Broker. Only events that pass this filter will be sent to the Subscriber. Defaults to subscribing to all events. |             |
| subscriber\* | pkg/duck.Destination | Subscriber is the addressable that receives events from the Broker that pass the Filter.                                                                                            |             |

\*: Required

#### Status

| Field              | Type       | Description                                                               | Constraints |
| ------------------ | ---------- | ------------------------------------------------------------------------- | ----------- |
| observedGeneration | int64      | The 'Generation' of the Broker that was last processed by the controller. |             |
| conditions         | Conditions | Trigger conditions.                                                       |             |
| subscriberURI      | apis.URL   | Address of the subscribing endpoint.                                      |             |

##### Conditions

- **Ready.** True when the Trigger is provisioned and configuration is ready to
  deliver events to the subscriber.
- **BrokerExists.** True when the Broker exists and is ready.
- **Subscribed.** True when the subscriber is subscribed to the Broker.

#### Events

- TriggerReconciled
- TriggerReconcileFailed
- TriggerUpdateStatusFailed

---

## kind: Broker

### group: eventing.knative.dev/v1beta1

_A Broker represents an event mesh. It logically receives events on its input
domain and forwards them to subscribers defined by one or more matching
Trigger._

### Object Schema

#### Spec

| Field    | Type                             | Description                                                                                                                                                  | Constraints |
| -------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | ----------- |
| config   | ObjectReference                  | Config is an ObjectReference to the configuration that specifies configuration options for this Broker. For example, this could be a pointer to a ConfigMap. |             |
| delivery | eventingduckv1beta1.DeliverySpec | Delivery is the delivery specification for Events within the Broker mesh. This includes things like retries, DLQ, etc.                                       |             |

#### Status

| Field              | Type        | Description                                                                                  | Constraints |
| ------------------ | ----------- | -------------------------------------------------------------------------------------------- | ----------- |
| observedGeneration | int64       | The 'Generation' of the Broker that was last processed by the controller.                    |             |
| conditions         | Conditions  | Broker conditions.                                                                           |             |
| address            | Addressable | Address of the endpoint which meets the [_Addressable_ contract](interfaces.md#addressable). |             |

##### Conditions

- **Ready.** True when the Broker is provisioned and ready to accept events.
- **Addressable.** True when the Broker has an resolved address in it's status.

#### Events

- BrokerReconciled
- BrokerUpdateStatusFailed

---

## kind: Channel

### group: messaging.knative.dev/v1beta1

_A Channel logically receives events on its input domain and forwards them to
its subscribers. A custom channel implementation (other than the default
\_InMemoryChannel_) can be referenced via the _channelTemplate_. The concrete
channel CRD can also be instantiated directly.\_

### Object Schema

#### Spec

| Field           | Type                | Description                         | Constraints |
| --------------- | ------------------- | ----------------------------------- | ----------- |
| channelTemplate | ChannelTemplateSpec | Specifies which channel CRD to use. | Immutable   |

#### Metadata

#### Status

| Field      | Type            | Description                                                                                  | Constraints |
| ---------- | --------------- | -------------------------------------------------------------------------------------------- | ----------- |
| address    | Addressable     | Address of the endpoint which meets the [_Addressable_ contract](interfaces.md#addressable). |             |
| conditions | Conditions      | Channel conditions.                                                                          |             |
| channel    | ObjectReference | The actual ObjectReference to the Channel CRD backing this Channel.                          |             |

##### Conditions

- **BackingChannelReady.** True when the backing Channel CRD is ready.
- **Addressable.** True when the Channel meets the Addressable contract and has
  a non-empty hostname.
- **Ready.** True when the backing channel is ready to accept events.

#### Events

- ChannelReconcileError
- ChannelReconciled
- ChannelReadinessChanged
- ChannelUpdateStatusFailed

### Life Cycle

| Action | Reactions                                                                                                                                                             | Constraints                                                                               |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Create | The Channel referenced will take ownership of the concrete Channel and begin provisioning the backing resources required for the Channel depending on implementation. | Only one Channel is allowed to be the Owner for a given Channel implementation (own CRD). |
| Update | The Channel will synchronize the Channel backing resources to reflect the update.                                                                                     |                                                                                           |
| Delete | The Channel will deprovision the backing resources if no longer required depending on the backing Channel implementation.                                             |                                                                                           |

---

## kind: Subscription

### group: messaging.knative.dev/v1beta1

_Describes a linkage between a Channel and a Callable and/or Addressable
channel._

### Object Schema

#### Spec

| Field                  | Type                             | Description                                                                       | Constraints        |
| ---------------------- | -------------------------------- | --------------------------------------------------------------------------------- | ------------------ |
| channel\*              | ObjectRef                        | The originating _Subscribable_ for the link.                                      | Must be a Channel. |
| subscriber<sup>1</sup> | Destination                      | Optional processing on the event. The result of subscriber will be sent to reply. |                    |
| reply<sup>1</sup>      | Destination                      | The continuation for the link.                                                    |                    |
| delivery               | eventingduckv1beta1.DeliverySpec | Delivery configuration.                                                           |                    |

\*: Required

1: At Least One(subscriber, reply)

#### Metadata

##### Owner References

- If a resource controller created this Subscription: Owned by the originating
  resource.

##### Conditions

- **Ready.**
- **FromReady.**
- **Resolved.** True if `channel`, `subscriber`, and `reply` all resolve into
  valid object references which implement the appropriate spec.

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

## Shared Object Schema

### ChannelTemplateSpec

| Field      | Type         | Description                                         | Constraints |
| ---------- | ------------ | --------------------------------------------------- | ----------- |
| kind       | String       | The backing channel CRD                             |             |
| apiVersion | String       | API version of backing channel CRD                  |             |
| spec       | RawExtension | Spec to be passed to backing channel implementation |             |

### Destination

| Field           | Type            | Description                                                                                          | Constraints                   |
| --------------- | --------------- | ---------------------------------------------------------------------------------------------------- | ----------------------------- |
| ref<sup>1</sup> | ObjectReference | The Subscription UID this SubscriberSpec was resolved from.                                          | Must adhere to _Addressable_. |
| uri<sup>1</sup> | apis.URL        | Either an absolute URL (if ref is not specified). Resolved using the base URI from ref if specified. | Must be a URL.                |

1: One or both (ref, uri), Required. If only uri is specified, it must be an
absolute URL. If both are specified, uri will be resolved using the base URI
retrieved from ref.

### duck.DeliverySpec

| Field          | Type          | Description                                                                                                                                      | Constraints |
| -------------- | ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------ | ----------- |
| deadLetterSink | Destination   | DeadLetterSink is the sink receiving event that could not be sent to a destination.                                                              |             |
| retry          | String        | Retry is the minimum number of retries the sender should attempt when sending an event before moving it to the dead letter sink.                 |             |
| backoffPolicy  | BackoffPolicy | BackoffPolicy is the retry backoff policy (linear, exponential).                                                                                 |             |
| backoffDelay   | String        | For linear policy, backoff delay is backoffDelay\*\<numberOfRetries>. For exponential policy, backoff delay is backoffDelay\*2^\<numberOfRetries>. |             |

### duck.SubscriberSpec

| Field         | Type     | Description                                                 | Constraints    |
| ------------- | -------- | ----------------------------------------------------------- | -------------- |
| uid           | String   | The Subscription UID this SubscriberSpec was resolved from. |                |
| subscriberURI | apis.URL | The URI name of the endpoint for the subscriber.            | Must be a URL. |
| replyURI      | apis.URL | The URI name of the endpoint for the reply.                 | Must be a URL. |

### ReplyStrategy

| Field     | Type      | Description                            | Constraints        |
| --------- | --------- | -------------------------------------- | ------------------ |
| channel\* | ObjectRef | The continuation Channel for the link. | Must be a Channel. |

\*: Required

### TriggerFilter

| Field      | Type              | Description                                                                         | Constraints |
| ---------- | ----------------- | ----------------------------------------------------------------------------------- | ----------- |
| attributes | map[string]string | A filter specifying which events match this trigger. Matches exactly on the fields. |             |
