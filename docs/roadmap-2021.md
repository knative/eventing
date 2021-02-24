# Eventing

## Next 3 Months (Short-term)

### Event Discovery

Event discovery have been a needed feature with many attempts to implement it. There's a need that users can discover various existing eventing components. One attempt to solve it was via the event registry and EventType CRD #929. The new Discovery API offers a more promising approach.

We need a more unified story for discovery in eventing that potentially leverages the Discovery API and aligns the event registry and EventType features accordingly.

**GitHub Issue:** https://github.com/knative/eventing/issues/4892

**Owner:** Matthis Wessendorf

### Spec Solidification

Review and update documented specification to reflect the existing implementation (v1 API).

**GitHub Issue:** https://github.com/knative/eventing/issues/4595

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

Simplify operations for developers using Knative by allowing addressable resources to have autotrigger sugar labels. AutoTrigger creates triggers based on annotations on a resource, and then assigns owner references between the Trigger and Addressable, allowing cleanup and a lot less operational complexity with the developer thinking about fewer resources

**GitHub Issue:** https://github.com/knative/eventing/issues/4547

**Owner:** Scott Nichols

### Remove v1beta1 API [eventing, flows, messaging]

Removal of v1beta1 APIs as per [our deprecation policy](
https://knative.dev/community/contributing/mechanics/release-versioning-principles/). We guarantee 9 months of support for v1beta1 APIs. This makes the release 18/05/2021 the earliest release where we can remove v1beta1 API, and we want to target that release.

**GitHub Issue:** https://github.com/knative/eventing/issues/4836

**Owner:** Ville Aikas

## Next 6 Months (Midterm)

### Protocol negotiation contract

Defined a standard that senders and receivers could use to communicate or negotiate protocol versions and features for a request, or a connection session to advance our capabilities, without leaving existing containers behind.

**Theme:** Multiple protocol and protocol option support

**GitHub Issue:** https://github.com/knative/eventing/issues/4868

**Owner:** Grant Rodgers

### Preflight protocol negotiation

[Description TBD]

**Theme:** Multiple protocol and protocol option support

**GitHub Issue:** https://github.com/knative/eventing/issues/4868

**Owner:** Grant Rodgers

## Icebox (Wishlist)

### Cross Namespace Eventing

[Description TBD]

**GitHub Issue:** TBD

**Owner:** Grant Rodgers

### Streaming Processing

The goal of this proposal is to build an efficient event mesh that allows stateless and stateful even processing. We want to empower end users to describe event flows, made by streams and processors.

**GitHub Issue:** https://github.com/knative/eventing/issues/4901

**Owner:** Lionel Villard

## Won’t Do

_These are the issues the working group decided to not work on whether for
being out of the WG scope or some other reason_

# Eventing Sources

## Next 3 Months (Short-term)

### Multi-Tenant Kafka Source

Introduce a multi-tenant KafkaSource implementation capable of handling more than one source instance at a time, typically all source instances in a namespace or all source instances in a cluster.

The goal of multi-tenant receive adapters is to minimize the cost of running sources that are barely (i.e. no or few processed events) used at a particular point in time.  

**Theme:** Multi-Tenant environment support

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/219

**Owner:** Lionel Villard

### GitHub Vanity domain support

The MT githubsource controller generates webhook of the form `http://githubsource-adapter.knative-sources.<domain>/<ns>/<name>` where `ns` and `name` corresponds the githubsource CR.

Instead it should generate webhook of the form `http://<name>.<ns>.<domain>.`

**Theme:** Multi-Tenant environment support

**GitHub Issue:** https://github.com/knative-sandbox/eventing-github/issues/65

**Owner:** Lionel Villard

### v1 for Core Sources

Let's promote PingSource to v1

**GitHub Issue:** https://github.com/knative/eventing/issues/4865

**Owner:** Lionel Villard

### v1 for Kafka Sources

Let's promote KafkaSource to v1

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/389

**Owner:** Matthis Wessendorf

## Next 6 Months (Midterm)

### Multi-Tenant RedisStream Source

Introduce a multi-tenant KafkaRedis implementation capable of handling more than one source instance at a time, typically all source instances in a namespace or all source instances in a cluster

**Theme:** Multi-Tenant environment support

**GitHub Issue:** https://github.com/knative-sandbox/eventing-redis/issues/95

**Owner:** Lionel Villard

## Icebox

## Won’t Do

_These are the issues the working group decided to not work on whether for
being out of the WG scope or some other reason_

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

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/413

**Owner:** Lionel Villard

## Icebox

### HTTP/2

Support HTTP/2 in eventing components supports transparently with possibly little/zero configuration in the full event flow

**GitHub Issue:** https://github.com/knative/eventing/issues/3312

**Owner:** Francesco Guardiani

## Won’t Do

_These are the issues the working group decided to not work on whether for being out of the WG scope or some other reason_
