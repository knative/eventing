# Interface Contracts

## Addressable

An **Addressable** resource receives events over a network transport
(currently only HTTP is supported). The _Addressable_ returns success when it
has successfully handled the event (for example, by committing it to stable
storage). When used as an _Addressable_, only the acknowledgement or return
code is used to determine whether the event was handled successfully. One
example of an _Addressable_ is a _Channel_.

### Control Plane

An **Addressable** resource MUST expose a `status.address.hostname` field.
The _hostname_ value is a cluster-resolvable DNS name which is capable of
receiving event deliveries. _Addressable_ resources may be referenced in the
`reply` section of a _Subscription_, and also by other custom resources acting
as an event Source.

### Data Plane

An **Addressable** resource will only respond to requests with success or
failure. Any payload (including a valid CloudEvent) returned to the sender
will be ignored. An _Addressable_ may receive the same event multiple times
even if it previously indicated success.

---

## Callable

A **Callable** resource represents an _Addressable_ endpoint which receives
events and optionally returns events to forward downstream. One example of a
_Callable_ is a function. Note that all _Callable_ resources are _Addressable_
(they accept an event and return a status code when completed), but not all
_Addressable_ resources are _Callable_.

### Control Plane

A **Callable** resource MUST expose a `status.address.hostname` field (like
_Addressable_). The _hostname_ value is a cluster-resolvable DNS name which is
capable of receiving event deliveries and returning a resulting event in the
reply.. _Callable_ resources may be referenced in the `subscriber` section of
a _Subscription_.

<!-- TODO(evankanderson):

What other properties separate a callable from an Addressable. We have talked
about using an annotation like `eventing.knative.dev/returnType = any` to
represent the return type of the _Callable_.

--->

### Data Plane

The **Callable** resource receives one event and returns zero or more events
in response. The returned events are not required to be related to the received
event. The _Callable_ should return a successful response if the event was
processed successfully.

The _Callable_ is not responsible for ensuring successful delivery of any
received or returned event. It may receive the same event multiple times even
if it previously indicated success.

---

_Navigation_:

- [Motivation and goals](motivation.md)
- [Resource type overview](overview.md)
- **Interface contracts**
- [Object model specification](spec.md)
