# Implementation Helper

This document supplements the official [spec](spec.md) with useful
implementation details for components implemented in this repo. Note that the
implementations for other components in other repos (Kafka, RabbitMQ, GCP, etc.)
may vary

- [Trigger](#kind-trigger)
- [Broker](#kind-broker)
- [Channel](#kind-channel)
- [Subscription](#kind-subscription)

## kind: Trigger

The `Trigger` specification is described in
[Object Model: Trigger](spec.md#kind-trigger).

### Readiness Sub-Conditions

- **BrokerReady.** True when the broker exists and is ready.
- **SubscriptionReady.** True when the subscriber is subscribed to the broker.
- **DependencyReady.** True when the sources the trigger depends on are ready.
- **SubscriberResolved.** True when the subscriber is resolved, i.e. the trigger
  can get its address.

### Events

- TriggerReconciled
- TriggerReconcileFailed
- TriggerUpdateStatusFailed

---

## kind: Broker

The `Broker` specification is described in
[Object Model: Broker](spec.md#kind-broker).

## Readiness Sub-Conditions

- **IngressReady.** True when the broker ingress is ready.
- **TriggerChannelReady.** True when the trigger channel is ready.
- **FilterReady.** True when the filter is ready.
- **Addressable.** True when the broker has a resolved address in its status.

### Events

- BrokerReconciled
- BrokerUpdateStatusFailed

---

## kind: Channel

The `Channel` specification is described in
[Object Model: Channel](spec.md#kind-channel).

### Readiness Sub-Conditions

- **BackingChannelReady.** True when the backing `Channel` CRD is ready.
- **Addressable.** True when the channel meets the
  [`Addressable`](interfaces.md#addressable) contract and has a non-empty
  hostname.

### Events

- ChannelReconcileError
- ChannelReconciled
- ChannelReadinessChanged
- ChannelUpdateStatusFailed

---

## kind: Subscription

The `Subscription` specification is described in
[Object Model: Subscription](spec.md#kind-subscription).

### Readiness Sub-Conditions

- **ReferencesResolved.** True when all the specified references (`channel`,
  `subscriber`, and `reply` ) have been successfully resolved.
- **AddedToChannel.** True when the controller has successfully added a
  subscription to the `spec.channel` resource.
- **ChannelReady.** True when the channel has marked the subscriber as ready.

### Events

- PublisherAcknowledged
- ActionFailed

---
