# Delivery Design

This document synthesizes the
[error handling design document](https://docs.google.com/document/d/1qRrzGoHJQO-oc5p-yRK8IRfugd-FM_PXyM7lN5kcqks).

## Delivery aspects

Each specification for _Sources_ and _Channels_ define some more fine-grained
delivery mechanism around their data plane. For details consult the respective
specifications.

- [Source Delivery](../spec/sources.md#source-event-delivery)
- [Channel Delivery](../spec/channel.md#data-plane)

## Problem

Sending events can fail for a variety of reasons (downstream system is down,
application logic rejects the invalid event, a runtime exception occurs, etc...)
but right now there is no way to control or define the expected behavior in
these situations.

## Requirements

- Be able to handle events that failed to be delivered
  - to channel subscribers.
  - to source sink.
  - to broker/triggers.
- Be able to identify events that could not be delivered (Observability)
- Be able to leverage existing error handling mechanisms provided by the
  underlying platform (eg. RabbitMQ dead letter exchange, Amazon SQS dead letter
  queue, Azure Service Bus dead letter queue, etc...).
- Be able to redirect of error'ed events from a channel.

### Out of Scope

- Security: No security model has been defined yet at the Knative eventing
  level. Native brokers might define their own security model and Knative
  eventing should not prevent using it.

## Day 1 Proposal

### Dead Letter Sink

Channels are responsible for forwarding received events to subscribers. When
they fail to do so, they are responsible for sending the failing events to an
**dead letter sink**, potentially one per subscriber.

Similarly Event Sources are responsible for sending events to a sink and when
they fail to do so, they are responsible for sending the failing events to an
**dead letter sink**.

The dead letter sink can be a channel but it does not have to.

### Dead-Letter Channel

Knative Channel implementations may leverage existing platform native error
handling support they might provide, like
[_Dead Letter Channel_](https://www.enterpriseintegrationpatterns.com/patterns/messaging/DeadLetterChannel.html),
to forward failed events from their _Dead Letter Channel_ to the configured
error sink(s).

### Retry

Channel implementations, event sources and brokers should retry sending events
before redirecting them to the dead letter sink. While there are many different
ways to implement the retry logic (immediate retry, retry queue, etc...),
implementations should rely on a common set of configuration parameters, such as
the number of retries and the interval between retries.

### Delivery Specification

The goal of this delivery specification is to formally define the vocabulary
related to capabilities defined above (dead letter sink, dead-letter queues and
retry) to provide consistency across all Knative event sources, channel
implementations and brokers.

The minimal delivery specification looks like this:

```go

// DeliverySpec contains the delivery options for event senders,
// such as channelable and source.
type DeliverySpec struct {
	// DeadLetterSink is the sink receiving events that couldn't be sent to
	// a destination.
	// +optional
	DeadLetterSink *duckv1.Destination `json:"deadLetterSink,omitempty"`

	// Retry is the minimum number of retries the sender should attempt when
	// sending an event before moving it to the dead letter sink
	// +optional
	Retry *int32 `json:"retry,omitempty"`

	// BackoffPolicy is the retry backoff policy (linear, exponential)
	// +optional
	BackoffPolicy *BackoffPolicyType `json:"backoffPolicy,omitempty"`

	// BackoffDelay is the delay before retrying.
	// More information on Duration format:
	//  - https://www.iso.org/iso-8601-date-and-time-format.html
	//  - https://en.wikipedia.org/wiki/ISO_8601
	//
	// For linear policy, backoff delay is backoffDelay*<numberOfRetries>.
	// For exponential policy, backoff delay is backoffDelay*2^<numberOfRetries>.
	// +optional
	BackoffDelay *string `json:"backoffDelay,omitempty"`
}

// BackoffPolicyType is the type for backoff policies
type BackoffPolicyType string

const (
	// Linear backoff policy
	BackoffPolicyLinear BackoffPolicyType = "linear"

	// Exponential backoff policy
	BackoffPolicyExponential BackoffPolicyType = "exponential"
)
```

Channel, brokers and event sources are not required to support all these
capabilities and are free to add more delivery options.

### Exposing underlying DLC

Channel implementation supporting dead letter channel should advertise it in
their status.

```go

// DeliveryStatus contains the Status of an object supporting delivery options.
type DeliveryStatus struct {
	// DeadLetterChannel is a KReference that is the reference to the native, platform specific channel
	// where failed events are sent to.
	// +optional
	DeadLetterChannel *duckv1.KReference `json:"deadLetterChannel,omitempty"`
}
```

### Error events

The error event is the original events annotated with various CloudEvents
attributes, eg. to be able to tell why the event could not be delivered.

Note that multiple copies of the same event can be sent to the error sink due to
multiple subscription failures.

Brokers might decide to change the event type before reposting the failed event
into the broker. This could be done by having a special error sink specific to
broker.

### CloudEvent extensions

(might move to the CloudEvent spec repository)

Here a possible set of CloudEvent extensions:

- deadlettersubscriberuri: The URI of the subscriber
- deadletterreason: The reason for dead lettering the event
- deadletterretry: How many times the channel tried to send the event
