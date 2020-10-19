# Knative Eventing Data Plane Contracts

## Introduction

Developers using Knative Eventing need to know what is supported for delivery to
user provided components that receive events. Knative Eventing defines contract
for data plane components and we have listed them here.

## Conformance

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in RFC2119.

## Data plane contract for Sinks

A **Sink** MUST be able to handle duplicate events.

A **Sink** is an [_addressable_](./interfaces.md#addressable) resource that
takes responsibility for the event. A Sink could be a consumer of events, or
middleware. A Sink MUST be able to receive CloudEvents over HTTP and HTTPS.

A **Sink** MAY be [_callable_](./interfaces.md#callable) resource that
represents an Addressable endpoint which receives an event as input and
optionally returns an event to forward downstream.

Almost every component in Knative Eventing may be a Sink providing
composability.

Every Sink MUST support HTTP Protocol Binding for CloudEvents
[version 1.0](https://github.com/cloudevents/spec/blob/v1.0/http-protocol-binding.md)
and
[version 0.3](https://github.com/cloudevents/spec/blob/v0.3/http-transport-binding.md)
with restrictions and extensions specified below.

### HTTP Support

This section adds restrictions on
[requirements in HTTP Protocol Binding for CloudEvents](https://github.com/cloudevents/spec/blob/v1.0/http-protocol-binding.md#12-relation-to-http).

Sinks MUST accept HTTP requests with POST method and MAY support other HTTP
methods. If a method is not supported Sink MUST respond with HTTP status code
`405 Method Not Supported`. Non-event requests (e.g. health checks) are not
constrained.

The URL used by a Sink MUST correspond to a single, unique endpoint at any given
moment in time. This MAY be done via the host, path, query string, or any
combination of these. This mapping is handled exclusively by the
[Addressable control-plane](./interfaces.md#control-plane) exposed via the
`status.address.url`.

If an HTTP request's URL does not correspond to an existing endpoint, then the
Sink MUST respond with `404 Not Found`.

Every non-Callable Sink MUST respond with `202 Accepted` if the request is
accepted.

If Sink is Callable it MAY respond with `200 OK` and a single event in the HTTP
response. A returned event is not required to be related to the received event.
The Callable should return a successful response if the event was processed
successfully. If there is no event to send back then Callable Sink MUST respond
with 2xx HTTP and with empty body.

If a Sink receives a request and is unable to parse a valid CloudEvent, then it
MUST respond with `400 Bad Request`.

### Content Modes Supported

A Sink MUST support `Binary Content Mode` and `Structured Content Mode` as
described in
[HTTP Message Mapping section of HTTP Protocol Binding for CloudEvents](https://github.com/cloudevents/spec/blob/master/http-protocol-binding.md#3-http-message-mapping)

A Sink MAY support `Batched Content Mode` but that mode is not used in Knative
Eventing currently (that may change in future).

### Retries

Sinks should expect that retries and accept possibility that duplicate events
may be delivered.

### Error handling

If Sink is not returning HTTP success header (200 or 202) then the event may be
sent again. If the event can not be delivered then some sources of events (such
as Knative sources, brokers or channels) MAY support
[dead letter sink or channel](../delivery/README.md) for events that can not be
delivered.

### Observability

CloudEvents received by Sink MAY have
[Distributed Tracing Extension Attribute](https://github.com/cloudevents/spec/blob/v1.0/extensions/distributed-tracing.md).

### Data plane contract for Sources

See [Source Delivery specification](../spec/sources.md#source-event-delivery)
for details.

### Data plane contract for Channels

See [Channel Delivery specification](../spec/channel.md#data-plane) for details.

### Data plane contract for Brokers

See [Broker Delivery specification](../spec/broker.md)

## Changelog

- 2020-04-20: `0.13.x release`: initial version that documents common contract
  for sinks, sources, channels and brokers.
