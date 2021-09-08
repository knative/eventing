### Roadmaps
- [Eventing Working Group](#Eventing)
- [Eventing Sources Working Group](#Eventing-Sources)
- [Eventing Kafka Working Group](#Eventing-Kafka)

# Eventing

## Next 3 Months (Short-term)

### Spec Solidification

Review and update documented specification to reflect the existing
implementation (v1 API).

**GitHub Issue:** https://github.com/knative/eventing/issues/4595

**Owner:** Evan Anderson

### Knative Eventing 1.0 Ready

Make the current feature set of Eventing ready for a 1.0 release.

**GitHub Issue** https://github.com/knative/eventing/issues/5039

**Owner:** Ahmed Abdalla

### Better Trigger Filtering

Trigger filtering today is limited. We need more expressive filtering.

**GitHub Issue:** https://github.com/knative/eventing/issues/3359

**Owner:** Ahmed Abdalla

## Next 6 Months (Midterm)

_Currently the Working Group is focused on v1.0 and conformance which left no room to breath and plan for longer period_

## Icebox (Wishlist)

### Event Discovery

Event discovery have been a needed feature with many attempts to implement it.
There's a need that users can discover various existing eventing components. One
attempt to solve it was via the event registry and EventType CRD #929. The new
Discovery API offers a more promising approach.

We need a more unified story for discovery in eventing that potentially
leverages the Discovery API and aligns the event registry and EventType features
accordingly.

**GitHub Issue:** https://github.com/knative/eventing/issues/4892

**Owner:** Matthis Wessendorf


### Cross Namespace Eventing

[Description TBD]

**GitHub Issue:** TBD

**Owner:** Grant Rodgers

### Streaming Processing

The goal of this proposal is to build an efficient event mesh that allows
stateless and stateful even processing. We want to empower end users to describe
event flows, made by streams and processors.

**GitHub Issue:** https://github.com/knative/eventing/issues/4901

**Owner:** Lionel Villard

### Protocol negotiation contract

Defined a standard that senders and receivers could use to communicate or
negotiate protocol versions and features for a request, or a connection session
to advance our capabilities, without leaving existing containers behind.

**Theme:** Multiple protocol and protocol option support

**GitHub Issue:** https://github.com/knative/eventing/issues/4868

**Owner:** Grant Rodgers

### Preflight protocol negotiation

[Description TBD]

**Theme:** Multiple protocol and protocol option support

**GitHub Issue:** https://github.com/knative/eventing/issues/4868

**Owner:** Grant Rodgers


### Autotrigger Sugar Controller

Simplify operations for developers using Knative by allowing addressable
resources to have autotrigger sugar labels. AutoTrigger creates triggers based
on annotations on a resource, and then assigns owner references between the
Trigger and Addressable, allowing cleanup and a lot less operational complexity
with the developer thinking about fewer resources

**GitHub Issue:** https://github.com/knative/eventing/issues/4547

**Owner:** Scott Nichols

### HTTP/2

Support HTTP/2 in eventing components supports transparently with possibly
little/zero configuration in the full event flow

**GitHub Issue:** https://github.com/knative/eventing/issues/3312

## Won’t Do

_These are the issues the working group decided to not work on whether for being
out of the WG scope or some other reason_

# Eventing Sources

## Next 3 Months (Short-term)


### GitHub Vanity domain support

The MT githubsource controller generates webhook of the form
`http://githubsource-adapter.knative-sources.<domain>/<ns>/<name>` where `ns`
and `name` corresponds the githubsource CR.

Instead it should generate webhook of the form `http://<name>.<ns>.<domain>.`

**Theme:** Multi-Tenant environment support

**GitHub Issue:** https://github.com/knative-sandbox/eventing-github/issues/65

**Owner:** Lionel Villard

### v1 for Core Sources

Let's promote PingSource to v1

**GitHub Issue:** https://github.com/knative/eventing/issues/4865

**Owner:** Lionel Villard


## Next 6 Months (Midterm)

### Multi-Tenant RedisStream Source

Introduce a multi-tenant Redis implementation capable of handling more than
one source instance at a time, typically all source instances in a namespace or
all source instances in a cluster

**Theme:** Multi-Tenant environment support

**GitHub Issue:** https://github.com/knative-sandbox/eventing-redis/issues/95

**Owner:** Lionel Villard

## Icebox
_Wishlist items that are part of the working group scope but have no one to work on_

## Won’t Do

_These are the issues the working group decided to not work on whether for being
out of the WG scope or some other reason_


# Eventing Kafka

## Next 3 Months (Short-term)

### v1 for Kafka Sources

Let's promote KafkaSource to v1

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/389

**Owner:** Matthis Wessendorf (TBD)

### Multi-Tenant Kafka Source

Introduce a multi-tenant KafkaSource implementation capable of handling more
than one source instance at a time, typically all source instances in a
namespace or all source instances in a cluster.

The goal of multi-tenant receive adapters is to minimize the cost of running
sources that are barely (i.e. no or few processed events) used at a particular
point in time.

**Theme:** Multi-Tenant environment support

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/219

**Owner:** Lionel Villard

### Kafka Code Share Improvements

KafkaChannel code refactorings to increase code sharing between the two
implementations

**Theme:** Kafka Code Share Improvements

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/386

**Owner:** Matthis Wessendorf (TBD)


### KafkaSource Tenant Targeted Logging

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

## Next 6 Months (Midterm)

### KafkaChannel Unification

Desired exit goal: One backing implemetation for the `KafkaChannel` API.

**Theme:** Kafka Code Share Improvements

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/386

**Owner:** Matthis Wessendorf (TBD)


### Various QoS guarantees for Kakfa

Today only at-least-once and ordered is supported by the Kafka-backed channel
implementation. At-most-once and unordered should also be supported.

**GitHub Issue:** https://github.com/knative-sandbox/eventing-kafka/issues/413

**Owner:** Lionel Villard

### Docs Review and Polishing

Docs for Kafka compontens need to be reviewed, polished and refactored to give a seamless experience to Knative Eventing users looking for how to use Kafka based eventing compontens.

**GitHub Issue** TBD

## Icebox
_Wishlist items that are part of the working group scope but have no one to work on_

## Won’t Do

_These are the issues the working group decided to not work on whether for being
out of the WG scope or some other reason_
