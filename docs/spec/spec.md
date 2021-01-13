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

### group: eventing.knative.dev/v1

_A `Trigger` represents a request to have events delivered to a subscriber from
a broker's event pool._

### Object Schema

#### Metadata

Standard Kubernetes
[meta.v1/ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta)
resource.

#### Spec

The `TriggerSpec` defines the desired state for the `Trigger`.

| Field Name   | Field Type                                  | Requirement | Description                                                                                                               | Default Value           |
| ------------ | ------------------------------------------- | ----------- | ------------------------------------------------------------------------------------------------------------------------- | ----------------------- |
| `broker`     | `string`                                    | Required    | The broker that this trigger receives events from.                                                                        | 'default'               |
| `filter`     | [`TriggerFilter`](#triggerfilter)           | Optional    | The filter to apply against all events from the broker. Only events that pass this filter will be sent to the subscriber. | subscribe to all events |
| `subscriber` | [`duckv1.Destination`](#duckv1.destination) | Required    | The addressable that receives events from the broker that pass the filter.                                                |                         |

#### Status

The `TriggerStatus` represents the current state of the `Trigger`.

| Field Name           | Field Type                            | Requirement | Description                                                                            | Constraints |
| -------------------- | ------------------------------------- | ----------- | -------------------------------------------------------------------------------------- | ----------- |
| `observedGeneration` | `int`                                 | Optional    | The 'Generation' of the `Trigger` that was last processed by the controller.           |             |
| `conditions`         | [`[]apis.Condition`](#apis.condition) | Optional    | Trigger conditions. The latest available observations of the resource's current state. |             |
| `annotations`        | `map[string]string`                   | Optional    | Fields to save additional state as well as convey more information to the user.        |             |
| `subscriberURI`      | [`apis.URL`](#apis.url)               | Required    | The resolved address of the receiver for this trigger.                                 |             |

##### Conditions

- **Ready.** True when the trigger is provisioned and ready to deliver events to
  the subscriber.

---

## kind: Broker

### group: eventing.knative.dev/v1

_A `Broker` collects a pool of events that are consumable using triggers.
Brokers provide a discoverable endpoint (`status.address`) for event delivery
that senders can use with minimal knowledge of the event routing strategy.
Subscribers use triggers to request delivery of events from a broker's pool to a
specific URL or `Addressable` endpoint._

### Object Schema

#### Metadata

Standard Kubernetes
[meta.v1/ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta)
resource.

#### Spec

The `BrokerSpec` defines the desired state for the `Broker`.

| Field Name | Field Type                                                    | Requirement | Description                                                                                                  | Constraints |
| ---------- | ------------------------------------------------------------- | ----------- | ------------------------------------------------------------------------------------------------------------ | ----------- |
| `config`   | [`duckv1.KReference`](#duckv1.kreference)                     | Optional    | Reference to the configuration options for this broker. For example, this could be a pointer to a ConfigMap. |             |
| `delivery` | [`eventingduckv1.DeliverySpec`](#eventingduckv1.deliveryspec) | Optional    | The delivery specification for events within the broker. This includes things like retries, DLQ, etc.        |             |

#### Status

The `BrokerStatus` represents the current state of the `Broker`.

| Field Name           | Field Type                                  | Requirement | Description                                                                                                               | Constraints |
| -------------------- | ------------------------------------------- | ----------- | ------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `observedGeneration` | `int`                                       | Optional    | The 'Generation' of the `Broker` that was last processed by the controller.                                               |             |
| `conditions`         | [`[]apis.Condition`](#apis.condition)       | Optional    | Broker conditions. The latest available observations of the resource's current state.                                     |             |
| `annotations`        | `map[string]string`                         | Optional    | Fields to save additional state as well as convey more information to the user.                                           |             |
| `address`            | [`duckv1.Addressable`](#duckv1.addressable) | Required    | The exposed endpoint URI for getting events delivered into the broker. The broker is [`Addressable`](duckv1.addressable). |             |

##### Conditions

- **Ready.** True when the broker is provisioned and ready to accept events.

---

## kind: Channel

### group: messaging.knative.dev/v1

_A Channel logically receives events on its input domain and forwards them to
its subscribers. A custom channel implementation (other than the default
\_InMemoryChannel_) can be referenced via the _channelTemplate_. The concrete
channel CRD can also be instantiated directly.\_

### Object Schema

#### Metadata

Standard Kubernetes
[meta.v1/ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta)
resource.

#### Spec

The `ChannelSpec` defines which subscribers have expressed interest in receiving
events from this `Channel`. It also defines the ChannelTemplate to use in order
to create the CRD Channel backing this Channel.

| Field Name        | Field Type                                    | Description                         | Constraints               |
| ----------------- | --------------------------------------------- | ----------------------------------- | ------------------------- |
| `channelTemplate` | [`ChannelTemplateSpec`](#ChannelTemplateSpec) | Specifies which channel CRD to use. | Immutable after creation. |

#### Status

The `ChannelStatus` represents the current state of the `Channel`.

| Field Name           | Field Type                                  | Requirement | Description                                                                                                            | Constraints |
| -------------------- | ------------------------------------------- | ----------- | ---------------------------------------------------------------------------------------------------------------------- | ----------- |
| `observedGeneration` | `int`                                       | Optional    | The 'Generation' of the `Channel` that was last processed by the controller.                                           |             |
| `conditions`         | [`[]apis.Condition`](#apis.condition)       | Optional    | Channel conditions. The latest available observations of the resource's current state.                                 |             |
| `annotations`        | `map[string]string`                         | Optional    | Fields to save additional state as well as convey more information to the user.                                        |             |
| `address`            | [`duckv1.Addressable`](#duckv1.addressable) | Required    | Address of the endpoint (as an URI) for getting events delivered into the channel.                                     |             |
| `subscribers`        | [`[]SubscriberStatus`](#subscriberstatus)   | Required    | The list of statuses for each of the channel's subscribers.                                                            |             |
| `deadLetterChannel`  | [`duckv1.KReference`](#duckv1.kreference)   | Optional    | Reference set by the channel when it supports native error handling via a channel. Failed messages are delivered here. |             |
| `channel`            | [`duckv1.KReference`](#duckv1.kreference)   | Required    | Reference to the `Channel` CRD backing this channel.                                                                   |             |

##### Conditions

- **Ready.** True when the channel is ready to accept events.

### Life Cycle

| Action | Reactions                                                                                                                                                                   | Constraints                                                                               |
| ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| Create | The `Channel` referenced will take ownership of the concrete `Channel` and begin provisioning the backing resources required for the `Channel` depending on implementation. | Only one Channel is allowed to be the Owner for a given Channel implementation (own CRD). |
| Update | The `Channel` will synchronize the `Channel` backing resources to reflect the update.                                                                                       |                                                                                           |
| Delete | The `Channel` will deprovision the backing resources if no longer required depending on the backing Channel implementation.                                                 |                                                                                           |

---

## kind: Subscription

### group: messaging.knative.dev/v1

_Subscription routes events received on a Channel to a DNS name._

### Object Schema

#### Metadata

Standard Kubernetes
[meta.v1/ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.18/#objectmeta-v1-meta)
resource.

#### Spec

The `SubscriptionSpec` specifies the `Channel` for incoming events, a subscriber
target for processing those events and where to put the result of this
processing. Only the channel where the events are coming from is always
required. You can optionally only process the events (results in no output
events) by leaving out the result. You can also perform an identity
transformation on the incoming events by leaving out the subscriber and only
specifying the result.

| Field Name   | Field Type                                                    | Requirement               | Description                                                                              | Constraints                   |
| ------------ | ------------------------------------------------------------- | ------------------------- | ---------------------------------------------------------------------------------------- | ----------------------------- |
| `channel`    | [`corev1.ObjectReference` ](#corev1.ObjectReference)          | Required                  | Reference to a channel that will be used to create the subscription.                     | Must be a Channel. Immutable. |
| `subscriber` | [`duckv1.Destination`](#duckv1.Destination)                   | Required if no reply      | Optional function for processing events. The result of subscriber will be sent to reply. |                               |
| `reply`      | [`duckv1.Destination`](#duckv1.Destination)                   | Required if no subscriber | Optionally specifies how to handle events returned from the subscriber target.           |                               |
| `delivery`   | [`eventingduckv1.DeliverySpec`](#eventingduckv1.DeliverySpec) | Optional                  | Delivery configuration.                                                                  |                               |

##### Owner References

- If a resource controller created this Subscription: Owned by the originating
  resource.

#### Status

The `SubscriptionStatus` represents the current state of the `Subscription`.

| Field Name             | Field Type                                                                          | Requirement | Description                                                                                 | Constraints |
| ---------------------- | ----------------------------------------------------------------------------------- | ----------- | ------------------------------------------------------------------------------------------- | ----------- |
| `observedGeneration`   | `int`                                                                               | Optional    | The 'Generation' of the `Subscription` that was last processed by the controller.           |             |
| `conditions`           | [`[]apis.Condition`](#apis.condition)                                               | Optional    | Subscription conditions. The latest available observations of the resource's current state. |             |
| `annotations`          | `map[string]string`                                                                 | Optional    | Fields to save additional state as well as convey more information to the user.             |             |
| `physicalSubscription` | [`SubscriptionStatusPhysicalSubscription`](#SubscriptionStatusPhysicalSubscription) | Required    | The fully resolved values that this `Subscription` represents.                              |             |

##### Conditions

- **Ready.** True when the subscription is ready to deliver events to the
  subscriber.

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

### duckv1.Addressable

`Addressable` provides a generic mechanism for a custom resource definition to
indicate a destination for message delivery. `Addressable` is the schema for the
destination information. This is typically stored in the object's `status`, as
this information may be generated by the controller. More details in
[`Addressable` interface](interfaces.md#addressable).

| Field Name | Field Type              | Description                                 | Constraints     |
| ---------- | ----------------------- | ------------------------------------------- | --------------- |
| `url`      | [`apis.URL`](#apis.url) | The URI name of the endpoint for the reply. | Must be an URL. |

### apis.URL

`URL` is an alias of [`url.URL`](https://golang.org/pkg/net/url/#URL) . It has
custom JSON marshal methods that enable it to be used in Kubernetes CRDs such
that the CRD resource will have the URL, but operator code can work with the
`url.URL` struct.

### duckv1.KReference

`KReference` contains enough information to refer to another object.

| Field Name   | Field Type | Requirement | Description                                                                                                              | Default Value                        | Constraints |
| ------------ | ---------- | ----------- | ------------------------------------------------------------------------------------------------------------------------ | ------------------------------------ | ----------- |
| `kind`       | `string`   | Required    | [Kind of the referent.](https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds) |                                      |             |
| `namespace`  | `string`   | Optional    | [Namespace of the referent.](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/)              | defaulted to the object embedding it |             |
| `name`       | `string`   | Required    | [Name of the referent.](https://kubernetes.io/docs/concepts/oFURLverview/working-with-objects/names/#names)              |                                      |             |
| `apiVersion` | `string`   | Required    | API version of the referent.                                                                                             |                                      |             |

### duckv1.Destination

`Destination` represents a target of an invocation over HTTP.

| Field Name        | Field Type                                | Description                                                                                                                                                                        | Constraints                                                |
| ----------------- | ----------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- |
| `ref`<sup>1</sup> | [`duckv1.KReference`](#duckv1.kreference) | Reference to an [`duckv1.Addressable`](#duckv1.addressable).                                                                                                                       | Must adhere to [`duckv1.Addressable`](duckv1.addressable). |
| `uri`<sup>1</sup> | [`apis.URL`](#apis.url)                   | Either an absolute URL (non-empty scheme and non-empty host) pointing to the target or a relative URI. The relative URIs will be resolved using the base URI retrieved from `ref`. | Must be an URL.                                            |

1: One or both (ref, uri), Required. If only uri is specified, it must be an
absolute URL. If both are specified, uri will be resolved using the base URI
retrieved from ref.

### eventingduckv1.DeliverySpec

`DeliverySpec` contains the delivery options for event senders.

| Field Name       | Field Type                                  | Requirement | Description                                                                                                                                                       | Constraints |
| ---------------- | ------------------------------------------- | ----------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------- |
| `deadLetterSink` | [`duckv1.Destination`](#duckv1.Destination) | Optional    | The sink receiving event that could not be sent to a `Destination`.                                                                                               |             |
| `retry`          | `int`                                       | Optional    | The minimum number of retries the sender should attempt when sending an event before moving it to the dead letter sink (when specified) or discarded (otherwise). |             |
| `backoffPolicy`  | `string`                                    | Optional    | The retry backoff policy (`linear` or `exponential`).                                                                                                             |             |
| `backoffDelay`   | `string`                                    | Optional    | For linear policy, backoff delay is backoffDelay\*\<numberOfRetries>. For exponential policy, backoff delay is backoffDelay\*2^\<numberOfRetries>.                |             |

### SubscriberStatus

`SubscriberStatus` defines the status of a single subscriber to a `Channel`.

| Field Name           | Field Type               | Requirement | Description                                                  | Constraints |
| -------------------- | ------------------------ | ----------- | ------------------------------------------------------------ | ----------- |
| `uid`                | `types.UID`              | Optional    | UID is used to understand the origin of the subscriber.      |             |
| `observedGeneration` | `int`                    | Optional    | Generation of the origin of the subscriber with uid:UID.     |             |
| `ready`              | `corev1.ConditionStatus` | Required    | Status of the subscriber.                                    |             |
| `message`            | `string`                 | Optional    | A human readable message indicating details of Ready status. |             |

### SubscriptionStatusPhysicalSubscription

`SubscriptionStatusPhysicalSubscription` represents the fully resolved values
for a `Subscription`.

| Field Name          | Field Type              | Requirement | Description                                                    | Constraints |
| ------------------- | ----------------------- | ----------- | -------------------------------------------------------------- | ----------- |
| `subscriberURI`     | [`apis.URL`](#apis.url) | Required    | The fully resolved URI for `spec.subscriber`.                  |             |
| `replyURI`          | [`apis.URL`](#apis.url) | Required    | The fully resolved URI for the `spec.reply`.                   |             |
| `deadLetterSinkURI` | [`apis.URL`](#apis.url) | Required    | The fully resolved URI for the `spec.delivery.deadLetterSink`. |             |

### ChannelTemplateSpec

| Field Name   | Field Type             | Requirement | Description                                                  | Constraints |
| ------------ | ---------------------- | ----------- | ------------------------------------------------------------ | ----------- |
| `kind`       | `string`               | Optional    | The backing channel CRD                                      |             |
| `apiVersion` | `string`               | Optional    | API version of backing channel CRD                           |             |
| `spec`       | `runtime.RawExtension` | Optional    | Spec to be passed verbatim to backing channel implementation |             |

### TriggerFilter

| Field Name   | Field Type          | Requirement | Description                                                                                                                                                                                                                                                                 | Constraints                                                                                                                     |
| ------------ | ------------------- | ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- |
| `attributes` | `map[string]string` | Optional    | A map of context attribute names to values for filtering by equality. Each key in the map is compared with the equivalent key in the event context. An event passes the filter if all values are equal to the specified values. The value '' to indicate all strings match. | Nested context attributes are not supported as keys. Only string values are supported. Only exact matches will pass the filter. |

### apis.Condition

The Knative API uses the
[Kubernetes Conditions convention](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties)
to communicate errors and problems to the user. Each user-visible resource
described in Resource Overview MUST have a `conditions` field in `status`, which
must be a list of `Condition` objects of the following form (note that the
actual API object types may be named `FooCondition` to allow better code
generation and disambiguation between similar fields in the same `apiGroup`):

| Field Name           | Field Type                                            | Description                                                                                                                                 | Default Value         |
| -------------------- | ----------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- | --------------------- |
| `type`               | `string`                                              | The category of the condition, as a short, CamelCase word or phrase.<p>This is the primary key of the Conditions list when viewed as a map. | REQUIRED – No default |
| `status`             | Enum:<ul><li>"True"<li>"False"<li>"Unknown"</li></ul> | The last measured status of this condition.                                                                                                 | "Unknown"             |
| `reason`             | `string`                                              | One-word CamelCase reason for the condition's last transition.                                                                              | ""                    |
| `message`            | `string`                                              | Human-readable sentence describing the last transition.                                                                                     | ""                    |
| `severity`           | Enum:<ul><li>""<li>"Warning"<li>"Info"</li></ul>      | If present, represents the severity of the condition. An empty severity represents a severity level of "Error".                             | ""                    |
| `lastTransitionTime` | `Timestamp`                                           | Last update time for this condition.                                                                                                        | "" – may be unset     |

Additionally, the resource's `status.conditions` field MUST be managed as
follows to enable clients (particularly user interfaces) to present useful
diagnostic and error message to the user. In the following section, conditions
are referred to by their `type` (aka the string value of the `type` field on the
Condition).

1.  Each resource MUST have either a `Ready` condition (for ongoing systems) or
    `Succeeded` condition (for resources that run to completion) with
    `severity=""`, which MUST use the `True`, `False`, and `Unknown` status
    values as follows:

    1.  `False` MUST indicate a failure condition.
    1.  `Unknown` SHOULD indicate that reconciliation is not yet complete and
        success or failure is not yet determined.
    1.  `True` SHOULD indicate that the application is fully reconciled and
        operating correctly.

    `Unknown` and `True` are specified as SHOULD rather than MUST requirements
    because there may be errors which prevent serving which cannot be determined
    by the API stack (e.g. DNS record configuration in certain environments).
    Implementations are expected to treat these as "MUST" for factors within the
    control of the implementation.

1.  For non-`Ready` conditions, any conditions with `severity=""` (aka "Error
    conditions") must be aggregated into the "Ready" condition as follows:

    1.  If the condition is `False`, `Ready` MUST be `False`.
    1.  If the condition is `Unknown`, `Ready` MUST be `False` or `Unknown`.
    1.  If the condition is `True`, `Ready` may be any of `True`, `False`, or
        `Unknown`.

    Implementations MAY choose to report that `Ready` is `False` or `Unknown`
    even if all Error conditions report a status of `True` (i.e. there may be
    additional hidden implementation conditions which feed into the `Ready`
    condition which are not reported.)

1.  Non-`Ready` conditions with non-error severity MAY be surfaced by the
    implementation. Examples of `Warning` or `Info` conditions could include:
    missing health check definitions, scale-to-zero status, or non-fatal
    capacity limits.

Conditions type names should be chosen to describe positive conditions where
`True` means that the condition has been satisfied. Some conditions may be
transient (for example, `ResourcesAllocated` might change between `True` and
`False` as an application scales to and from zero). It is RECOMMENDED that
transient conditions be indicated with a `severity="Info"`.
