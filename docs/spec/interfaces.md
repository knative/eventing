# Interface Contracts

!Interface Contracts Overview](images/interface-contracts-overview.svg)

## Subscribable

A **Subscribable** resource will emit events that a _Subscription_ can direct
to a _Targetable_ or _Sinkable_ resource.

### Control Plane

A **Subscribable** resource may be referenced in the _from_ section of a
_Flow_. The **Subscribable** resource MUST expose a _channel_ field (an
ObjectReference) in its _status_ section. This _channel_ field MUST be
_Channelable_, and _channel_ MAY refer back to the **Subscribable** resource.

### Data Plane

A **Subscribable** resource is an event producer or forwarder, events that it
produces or forwards are delivered via its _status.channel_ resource.

## Channelable

A **Channelable** resource owns a list of subscribers for delivery of events.

### Control Plane

The **Channelable** resource has a list of _subscribers_ within the resources
_spec_. In practice, the resolved _subscription_ _call_ and _result_ endpoints
populate the Channelable's list of _subscribers_.

### Data Plane

**Channelable** resources will attempt delivery to each of they _subscribers_
at least once, and retry if the callie returns errors.

## Targetable

A **Targetable** resource represents a unit of work to be done on/to/with an
event. They allow for event modification and flow control without taking
ownership of the event. Typically a **Targetable** is a function.

### Control Plane

A **Targetable** resource MUST expose a _targetable.domainInternal_ field in
its _status_ section. The _domainInternal_ value is an internal domain name
that is capable of receiving event deliveries. **Targetable** resources may be
referenced in the _call_ section of a _Subscription_.

### Data Plane

The **Targetable** resource does not ownership of the event that was passed to
it, but it is allowed to modify the event by returning a response code of 200
and a mutated version back to the caller. The **Targetable** can prevent the
event from continuing to the next stage by returning 200 and an empty body. All
other response codes are considered an error and the caller will attempt to
call again.

## Sinkable

A **Sinkable** resource takes ownership of an event. Typically a **Sinkable**
is a Channel.

### Control Plane

A **Sinkable** resource MUST expose a _targetable.domainInternal_ field in its
_status_ section. The _domainInternal_ value is an internal domain name that is
capable of receiving event deliveries. **Sinkable** resources** **may be
referenced in the _result_ section of a _Subscription_.

### Data Plane

A **Sinkable** resource will only respond to requests by ACK/NACK. It must not
return events to the caller.
