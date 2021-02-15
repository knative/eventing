# Eventing

## Next 3 Months (Short-term)

### Event Discovery

[Description TBD]

**GitHub Issue:** TBD

**Owner:** Grant Rodgers

### Spec Solidification

[Description TBD]

**GitHub Issue:** TBD

**Owner:** Grant Rodgers

### Better Trigger Filtering

Trigger filtering today is limited. We need more expressive filtering.

**GitHub Issue:** https://github.com/knative/eventing/issues/3359

**Owner:** Francesco Guardiani

### Tenant Targeted Logging

Hosting Knative as a multi-tenant cloud offering, there is currently no means to
distinguish logging events that are necessary for servicing the hosted Knative
service and those that may be meaningful to a tenant consuming the service. The
tenant of the hosted service would like to see besides their hosted application
logging, pertinent log messages from Knative service to help diagnose their
issues. However, providing all logging messages from the Knative service
internals would be counterproductive for a tenant not familiar with Knative
internals

**Theme:** Multi-Tenant environment support

**GitHub Issue:** https://github.com/knative/eventing/issues/3299

**Owner:** Rick Rineholt

### Autotrigger Sugar Controller

[Description TBD]

**GitHub Issue:** https://github.com/knative/eventing/issues/4547

**Owner:** Scott Nichols

### Remove v1beta1 API [eventing, flows, messaging]

[Description TBD]

**GitHub Issue:** https://github.com/knative/eventing/issues/4836

**Owner:** Ville Aikas

## Next 6 Months (Midterm)

### Protocol negotiation contract

[Description TBD]

**Theme:** Multiple protocol and protocol option support

**GitHub Issue:** TBD

**Owner:** Grant Rodgers

### Preflight protocol negotiation

[Description TBD]

**Theme:** Multiple protocol and protocol option support

**GitHub Issue:** TBD

**Owner:** Grant Rodgers

## Icebox (Wishlist)

### Broker Performance Tests

[Description TBD]

**GitHub Issue:** TBD

**Owner:** TBD

### Cross Namespace Eventing

[Description TBD]

**GitHub Issue:** TBD

**Owner:** Grant Rodgers

### Streaming Processing

[Description TBD]

**GitHub Issue:** TBD

**Owner:** Lionel Villard

## Won’t Do

\_\_These are the issues the working group decided to not work on whether for
being out of the WG scope or some other reason

# Eventing Sources

## Next 3 Months (Short-term)

### Introduce and define Multi-tenant Sources

[Description TBD]

**Theme:** Multi-Tenant environment support

**GitHub Issue:** TBD

**Owner:** Scott Nichols

### Multi-Tenant Kafka Source

[Description TBD]

**Theme:** Multi-Tenant environment support

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/219

**Owner:** Lionel Villard

### GitHub Vanity domain support

[Description TBD]

**Theme:** Multi-Tenant environment support

**GitHub Issue:** https://github.com/knative-sandbox/eventing-github/issues/65

**Owner:** Lionel Villard

### Find Maintainers for source repos

[Description TBD]

**Theme:** No repo left behind

**GitHub Issue:** We should create one if we think this is valuable

**Owner:** Ville Aikas

### v1 for Core Sources

Let's promote PingSource to v1

**GitHub Issue:** https://github.com/knative/eventing/issues/4865

**Owner:** Lionel Villard

## Next 6 Months (Midterm)

### Multi-Tenant RedisStream Source

[Description TBD]

**Theme:** Multi-Tenant environment support

**GitHub Issue:** TBD

**Owner:** Lionel Villard

## Icebox

### Removal of v1alpha1 Sources

[Description TBD]

**GitHub Issue:** TBD

## Won’t Do

\_\_These are the issues the working group decided to not work on whether for
being out of the WG scope or some other reason

# Event Delivery

## Next 3 Months (Short-term)

### Kafka Code Share Improvements

KafkaChannel code refactorings to increase code sharing between the two implementations

**Theme:** Kafka Code Share Improvements

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/386

**Owner:** Matthias

## Next 6 Months (Midterm)

### KafkaChannel Unification

Desired exit goal: One backing implemetation for the `KafkaChannel` API.

**Theme:** Kafka Code Share Improvements

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/386

**Owner:** Matthias

### V1 for KafkaChannel

Promote the `KafkaChannel` to V1

**Theme:** Kafka Code Share Improvements

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/386

**Owner:** Matthias

### Various QoS guarantees for Kakfa

Today only at-least-once and ordered is supported by the Kafka-backed channel
implementation. At-most-once and unordered should also be supported.

**GitHub Issue:** TDB

**Owner:** Lionel Villard

## Icebox

### HTTP/2

[Description TBD]

**GitHub Issue:** TDB

**Owner:** Francesco Guardiani

## Won’t Do

\_\_These are the issues the working group decided to not work on whether for
being out of the WG scope or some other reason
