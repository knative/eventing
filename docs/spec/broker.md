# Knative Broker Specification

## Background

A _Broker_ specifies an _ingress_ to which _producers_ may produce events. A
_Trigger_ specifies a _subscriber_ that will receive events delivered to its
assigned Broker which match a specified _filter_.

## Conformance

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC2119.

## Control Plane

### Broker

Broker objects SHOULD include a Ready condition in their status.

The Broker SHOULD indicate Ready=True when its ingress is available to receive
events.

While a Broker is Ready, it SHOULD be a valid Addressable and its
`status.address.url` field SHOULD indicate the address of its ingress.

### Trigger

Triggers SHOULD include a Ready condition in their status.

The Trigger SHOULD indicate Ready=True when events can be delivered to its
subscriber.

While a Trigger is Ready, it SHOULD indicate its subscriber's URI via the
`status.subscriberUri` field.

Triggers MUST be assigned to exactly one Broker. Triggers SHOULD be assigned a
default Broker upon creation if no Broker is specified by the user.

A Trigger MAY be created before its assigned Broker exists. A Trigger SHOULD
progress to Ready when its assigned Broker exists and is Ready.

The attributes filter specifying a list of key-value pairs MUST be supported by
Trigger. Events that pass the attributes filter MUST include context or
extension attributes that match all key-value pairs exactly.

### Delivery Spec

Both BrokerSpec and TriggerSpec have a `Delivery` field of type
[`duck.DeliverySpec`](./spec.md#eventingduckv1deliveryspec). This field, among
the other features, allows the user to define the dead letter sink and retries.
The `BrokerSpec.Delivery` field is global across all the Triggers registered
with that particular Broker, while the `TriggerSpec.Delivery`, if configured,
fully overrides `BrokerSpec.Delivery` for that particular Trigger, hence:

- When `BrokerSpec.Delivery` and `TriggerSpec.Delivery` are both not configured,
  no delivery spec SHOULD be used.
- When `BrokerSpec.Delivery` is configured, but not the specific
  `TriggerSpec.Delivery`, then the `BrokerSpec.Delivery` SHOULD be used.
- When `TriggerSpec.Delivery` is configured, then `TriggerSpec.Delivery` SHOULD be
  used.

## Data Plane

### Ingress

A Broker SHOULD expose either an HTTP or HTTPS endpoint as ingress. It MAY
expose both.

The ingress endpoint(s) MUST conform to at least one of the following versions
of the specification:

- [CloudEvents 0.3 specification](https://github.com/cloudevents/spec/blob/v0.3/http-transport-binding.md)
- [CloudEvents 1.0 specification](https://github.com/cloudevents/spec/blob/v1.0/http-protocol-binding.md)

Other versions MAY be rejected. The usage of CloudEvents version 1.0 is
RECOMMENDED.

The Broker SHOULD NOT perform an upgrade of the produced event's CloudEvents
version. It SHOULD support both Binary Content Mode and Structured Content Mode
of the HTTP Protocol Binding for CloudEvents.

The HTTP(S) endpoint MAY be on any port, not just the standard 80 and 443.
Channels MAY expose other, non-HTTP endpoints in addition to HTTP at their
discretion (e.g. expose a gRPC endpoint to accept events).

Brokers MUST reject all HTTP produce requests with a method other than POST
responding with HTTP status code `405 Method Not Supported`. Non-event queueing
requests (e.g. health checks) are not constrained.

The Broker MUST respond with a 200-level HTTP status code if a produce request
is accepted.

If a Broker receives a produce request and is unable to parse a valid
CloudEvent, then it MUST reject the request with HTTP status code
`400 Bad Request`.

### Delivery

Delivered events must conform to the CloudEvents specification. All CloudEvent
attributes set by the producer, including the data and specversion attributes,
SHOULD be received at the subscriber identical to how they were received by the
Broker.

The Broker SHOULD support delivering events via Binary Content Mode or
Structured Content Mode of the HTTP Protocol Binding for CloudEvents.

Events accepted by the Broker SHOULD be delivered at least once to all
subscribers of all Triggers that:

1. are Ready when the produce request was received,
1. specify filters that match the event, and
1. exist when the event is able to be delivered.

Events MAY additionally be delivered to Triggers that become Ready after the
event was accepted.

Events MAY be enqueued or delayed between acceptance from a producer and
delivery to a subscriber.

The Broker MAY choose not to deliver an event due to persistent unavailability
of a subscriber or limitations such as storage capacity. The Broker SHOULD
attempt to notify the operator in this case. The Broker MAY forward these events
to an alternate endpoint or storage mechanism such as a dead letter queue.

If no ready Trigger would match an accepted event, the Broker MAY drop that
event without notifying the producer. From the producer's perspective, the event
was delivered successfully to zero subscribers.

If multiple Triggers reference the same subscriber, the subscriber MAY be
expected to acknowledge successful delivery of an event multiple times.

Events contained in delivery responses SHOULD be published to the Broker ingress
and processed as if the event had been produced to the Broker's addressable
endpoint.

Events contained in delivery responses that are malformed SHOULD be treated as
if the event delivery had failed. Reasoning being that if the event was being
transformed unsuccessfully (programming error for example) it should be treated
as a failure.

The subscriber MAY receive a confirmation that a reply event was accepted by the
Broker. If the reply event was not accepted, the initial event SHOULD be
redelivered to the subscriber.

### Observability

The Broker SHOULD expose a variety of metrics, including, but not limited to:

- Number of malformed produce requests (400-level responses)
- Number of accepted produce requests (200-level responses)
- Number of events delivered

Metrics SHOULD be enabled by default, with a configuration parameter included to
disable them if desired.

Upon receiving an event with context attributes defined in the
[CloudEvents Distributed Tracing extension](https://github.com/cloudevents/spec/blob/master/extensions/distributed-tracing.md),
the Broker SHOULD preserve that trace header on delivery to subscribers and on
reply events, unless the reply is sent with a different set of tracing
attributes. Forwarded trace headers SHOULD be updated with any intermediate
spans emitted by the broker.

Spans emitted by the Broker SHOULD follow the
[OpenTelemetry Semantic Conventions for Messaging Systems](https://github.com/open-telemetry/opentelemetry-specification/blob/master/specification/trace/semantic_conventions/messaging.md)
whenever possible. In particular, spans emitted by the Broker SHOULD set the
following attributes:

- messaging.system: "knative"
- messaging.destination: broker:name.namespace or trigger:name.namespace with
  the Broker or Trigger to which the event is being routed
- messaging.protocol: the name of the underlying transport protocol
- messaging.message_id: the event ID

## Conformance Tests

This test outline validates the conformance of a Broker implementation, with
some [exceptions](#exceptions).

1. Control Plane Tests
   1. A Trigger can be created before its Broker exists. That Trigger specifies
      an attributes filter.
   1. A Broker can be created (given valid configuration) and progresses to
      Ready.
   1. The Broker, once Ready, is a valid Addressable.
   1. The Trigger, once its Broker is Ready, progresses to Ready.
   1. The Trigger, once Ready, includes a status.subscriberUri field.
   1. A second Trigger, with a different but overlapping filter,, can be created
      and progresses to Ready.
1. Data Plane Tests
   1. Broker ingress can receive events in the following formats via HTTP:
      1. CloudEvents 0.3
      1. CloudEvents 1.0
      1. Structured mode
      1. Binary mode
   1. Broker ingress responds with:
      1. 2xx on valid event
      1. 400 on invalid event
   1. Broker ingress responds with 405 to any method except POST to publish URI
   1. Produce 2 events to ingress:
      1. Matching only first Trigger (event 1)
      1. Matching both first and second Trigger (event 2)
      1. With a known trace header
   1. First Trigger subscriber receives events 1 and 2:
      1. With original version
      1. With all originally specified attributes identical
      1. With the correct trace header
      1. Matching its filter
   1. And replies to event 1 with an event that matches only second trigger
      (event 3)
      1. Second Trigger subscriber receives events 2 and 3:
      1. And rejects events on first delivery, verifying events are redelivered
      1. With the correct trace header

### Exceptions

These aspects of the spec are not tested in this outline:

- **Replies that fail to be published cause initial message to be redelivered.**
  Requires implementation-specific setup to induce a failure.
- **Metrics support.** Currently there is no shared format that could be used to
  test support for metrics.

## Changelog

- `0.13.x release`: Initial version.
- Add conformance test outline.
