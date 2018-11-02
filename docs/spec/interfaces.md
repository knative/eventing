# Interface Contracts

## Targetable

A **Targetable** resource represents an endpoint that receives events and
optionally returns events to forward downstream. One example of a _Targetable_
is a function.

### Control Plane

A **Targetable** resource MUST expose a `status.targetable.domainInternal`
field. The _domainInternal_ value is an internal domain name that is capable of
receiving event deliveries. _Targetable_ resources may be referenced in the
`subscriber` section of a _Subscription_.

### Data Plane

The **Targetable** resource receives one event and returns zero or more events
in response. The returned events are not required to be related to the received
event. The _Targetable_ should return a successful response if the event was
processed successfully.

The _Targetable_ is not responsible for ensuring successful delivery of any
received or returned event. It may receive the same event multiple times even
if it previously indicated success.

---

## Sinkable

A **Sinkable** resource receives events and takes responsibility for further
delivery. Unlike _Targetable_, a _Sinkable_ cannot return events in its
response. One example of a _Sinkable_ is a _Channel_ as the target of a
_Subscription_'s `reply` field.

<!-- TODO(evankanderson):
I don't like this example, as it conflates two different things:

That Channel implements Sinkable.
That Subscription expects a Sinkable in its spec.reply.
I think it would be clearer to separate the two (and possibly cover the second
item only in the object specs).
-->

### Control Plane

A **Sinkable** resource MUST expose a `status.sinkable.domainInternal` field.
The _domainInternal_ value is an internal domain name that is capable of
receiving event deliveries. _Sinkable_ resources may be referenced in the
`reply` section of a _Subscription_, and also by other custom resources acting as an event Source.

### Data Plane

A **Sinkable** resource will only respond to requests with success of failure.
Any payload (including a valid CloudEvent) returned to the sender will be
ignored. It may receive the same event multiple times even if it previously
indicated success.

---

_Navigation_:

- [Motivation and goals](motivation.md)
- [Resource type overview](overview.md)
- **Interface contracts**
- [Object model specification](spec.md)
