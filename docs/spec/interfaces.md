# Interface Contracts

## Subscribable

A **Subscribable** resource contains a list of subscribers and is responsible
for delivering events to each of them. Subscriptions only allow Channels to be
Subscribable (via `spec.from` on the Subscription) at the moment, but this may
be revisited with future experience. _Channel_ as the target of a
_Subscription_'s `from` field.


### Control Plane

The **Subscribable** resource stores a list of resolved _Subscriptions_ in the
resource's `spec.subscribers` field. The Subscription Controller is responsible
for resolving any ObjectReferences (such as _call_ and _result_) in the
_Subscription_ to network addresses.

### Data Plane

**Subscribable** resources will attempt delivery to each of the _subscribers_
at least once, and retry if the subscriber returns errors.

<!--TODO(https://github.com/knative/eventing/issues/502) Expand the data plane definitions. -->

---

## Targetable

A **Targetable** resource represents an endpoint that receives events and
optionally returns events to forward downstream. One example of a _Targetable_
is a function.

### Control Plane

A **Targetable** resource MUST expose a _status.targetable.domainInternal_
field. The _domainInternal_ value is an internal domain name that is capable of
receiving event deliveries. _Targetable_ resources may be referenced in the
_call_ section of a _Subscription_.

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
_Subscription_'s _result_ field.

<!-- TODO(evankanderson):
I don't like this example, as it conflates two different things:

That Channel implements Sinkable.
That Subscription expects a Sinkable in its spec.from.
I think it would be clearer to separate the two (and possibly cover the second
item only in the object specs).
-->

### Control Plane

A **Sinkable** resource MUST expose a _status.sinkable.domainInternal_ field.
The _domainInternal_ value is an internal domain name that is capable of
receiving event deliveries. _Sinkable_ resources may be referenced in the
_result_ section of a _Subscription_, and also by other custom resources acting as an event Source.

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
