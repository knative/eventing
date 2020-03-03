# Knative Broker Specification

## Background

A _Broker_ specifies an _ingress_ to which _producers_ may produce events. A _Trigger_ specifies a _subscriber_ that will receive events delivered to its assigned Broker which match a specified _filter_.

## Conformance

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be interpreted as described in RFC2119.

## Control Plane

### Broker

Broker objects SHOULD include a Ready condition in their status.

The Broker SHOULD indicate Ready=True when its ingress is available to receive events.

While a Broker is Ready, it SHOULD be a valid Addressable and its `status.address.url` field SHOULD indicate the address of its ingress.

### Trigger

Triggers SHOULD include a Ready condition in their status.

The Trigger SHOULD indicate Ready=True when events can be delivered to its subscriber.

While a Trigger is Ready, it SHOULD indicate its subscriber's URI via the `status.subscriberUri` field.

Triggers MUST be assigned to exactly one Broker. Triggers SHOULD be assigned a default Broker upon creation if no Broker is specified by the user.

A Trigger MAY be created before its assigned Broker exists. A Trigger SHOULD progress to Ready when its assigned Broker exists and is Ready.

The attributes filter specifying a list of key-value pairs MUST be supported by Trigger. Events that pass the attributes filter MUST include context or extension attributes that match all key-value pairs exactly.

## Data Plane

### Ingress

A Broker SHOULD expose either an HTTP or HTTPS endpoint as ingress. It MAY expose both.

The ingress endpoint(s) MUST conform to at least one of the following versions of the specification:

* [CloudEvents 0.3 specification](https://github.com/cloudevents/spec/blob/v0.3/http-transport-binding.md)
* [CloudEvents 1.0 specification](https://github.com/cloudevents/spec/blob/v1.0/http-protocol-binding.md)

Other versions MAY be rejected. The usage of CloudEvents version 1.0 is RECOMMENDED.

The Broker SHOULD NOT perform an upgrade of the produced event's CloudEvents version. It SHOULD support both Binary Content Mode and Structured Content Mode of the HTTP Protocol Binding for CloudEvents.

The HTTP(S) endpoint MAY be on any port, not just the standard 80 and 443. Channels MAY expose other, non-HTTP endpoints in addition to HTTP at their discretion (e.g. expose a gRPC endpoint to accept events).

Brokers MUST reject all HTTP produce requests with a method other than POST responding with HTTP status code `405 Method Not Supported`. Non-event queueing requests (e.g. health checks) are not constrained.

The Broker MUST respond with a 200-level HTTP status code if a produce request is accepted.

If a Broker receives a produce request and is unable to parse a valid CloudEvent, then it MUST reject the request with HTTP status code `400 Bad Request`.

### Delivery

Delivered events must conform to the CloudEvents specification. All CloudEvent attributes set by the producer, including the data and specversion attributes, SHOULD be received at the subscriber identical to how they were received by the Broker.

The Broker SHOULD support delivering events via Binary Content Mode or Structured Content Mode of the HTTP Protocol Binding for CloudEvents.

Events accepted by the Broker SHOULD be delivered at least once to all subscribers of all Triggers that:

1. are Ready when the produce request was received,
1. specify filters that match the event, and
1. exist when the event is able to be delivered.

Events MAY additionally be delivered to Triggers that become Ready after the event was accepted.

Events MAY be enqueued or delayed between acceptance from a producer and delivery to a subscriber.

The Broker MAY choose not to deliver an event due to persistent unavailability of a subscriber or limitations such as storage capacity. The Broker SHOULD attempt to notify the operator in this case. The Broker MAY forward these events to an alternate endpoint or storage mechanism such as a dead letter queue.

If no ready Trigger would match an accepted event, the Broker MAY drop that event without notifying the producer. From the producer's perspective, the event was delivered successfully to zero subscribers.

If multiple Triggers reference the same subscriber, the subscriber MAY be expected to acknowledge successful delivery of an event multiple times.

Events contained in delivery responses SHOULD be published to the Broker ingress and processed as if the event had been produced to the Broker's addressable endpoint.

The subscriber MAY receive a confirmation that a reply event was accepted by the Broker. If the reply event was not accepted, the initial event SHOULD be redelivered to the subscriber.

### Observability

The Broker SHOULD expose a variety of metrics, including, but not limited to:

* Number of malformed produce requests (400-level responses)
* Number of accepted produce requests (200-level responses)
* Number of events delivered

Metrics SHOULD be enabled by default, with a configuration parameter included to disable them if desired.

Upon receiving an event with context attributes defined in the [CloudEvents Distributed Tracing extension](https://github.com/cloudevents/spec/blob/master/extensions/distributed-tracing.md), the Broker SHOULD preserve that trace header on delivery to subscribers and on reply events, unless the reply is sent with a different set of tracing attributes.

## Changelog

* `0.13.x release`: Initial version.

