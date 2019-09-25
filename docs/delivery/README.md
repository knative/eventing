# Delivery Design

This document synthetizes the [error handling design document](https://docs.google.com/document/d/1qRrzGoHJQO-oc5p-yRK8IRfugd-FM_PXyM7lN5kcqks).

## Problem

Sending events can fail for a variety of reasons (downstream system is down, application logic rejects the invalid message, a runtime exception occurs, etc...) but right now there is no way to control or define the expected behavior in these situations.

## Requirements

* Be able to handle events that failed to be delivered
    * to channel subscribers.
    * to source sink.
    * to broker/triggers.
* Be able to identify a message couldn’t be delivered (Observability)
* Be able to leverage existing native error handling mechanisms (eg. dead letter queues).
* Be able to redirect of error'ed events from a channel.

### Out of Scope

* Security: No security model has been defined yet at the Knative eventing level. Native brokers might define their own security model and Knative eventing should not prevent using it.

## Proposal

### Error Sink

Channels are responsible for forwarding received messages to subscribers. When they fail to do so, they are responsible for sending the failing messages to an **error sink**. It could be a channel but it does not have to.

Similarly Event Sources are responsible sending events to a sink and when they fail to do so, they are responsible for sending the failing events to an **error sink**.

### Dead-Letter Channel

Channel implementations might leverage the existing native error handling support they provide, usually a dead letter channel, to forward failed messages to the error sink. In that case, the error sink might be realized by creating a subscription on the error channel.

### Retry

Typically channel implementations and event sources retry sending messages before redirecting them to the error sink.
While there are many different ways to implement the retry logic
(immediate retry, retry queue, etc...), implementations usually
rely on a common set of configuration parameters, such as
the number of retries and the interval between retries.

### Delivery Specification

The goal of this delivery specification is to formally define the vocabulary related to capabilities defined above (error sink, dead-letter queues and retry) to provide consistency accross all Knative event sources, channels and brokers.

The minimal delivery specification looks like this:

```go

// DeliverySpec contains the delivery options for event senders,
// such as channelable and source.
type DeliverySpec struct {
	// ErrorSink is the sink receiving messages that couldn't be sent to
	// a destination.
	// +optional
	ErrorSink *apisv1alpha1.Destination `json:"errorSink,omitempty"`

	// Retry is the mimimum number of retries the sender should attempt when
	// sending a message before moving it to the error sink.
	// +optional
	Retry *int32 `json:"retry,omitempty"`

	// BackoffPolicy is the retry backoff policy (linear, exponential)
	// +optional
	BackoffPolicy *BackoffPolicyType `json:"backoffPolicy,omitempty"`

	// BackoffDelay is the delay before retrying.
	// More information on Duration format: https://www.ietf.org/rfc/rfc3339.txt
	//
	// For linear policy, backoff delay is the time interval between retries.
	// For exponential policy , backoff delay is backoffDelay*10^<numberOfRetries>
	// +optional
	BackoffDelay *string
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

Channel, brokers and event sources  are not required to support all this capabilities and are free to add more delivery options.

### Exposing underlying DLC

Channels supporting dead letter queue should advertise it in their status.

```go

// DeliverStatus contains the Status of an object supporting delivery options.
type DeliverStatus struct {
	// ErrorChannel is the reference to the channel where failed events are sent to.
	// +optional
	ErrorChannel *corev1.ObjectReference `json:"errorChannel,omitempty"`
}
```

### Error messages

The error message is the original messages annotated with various CloudEvents attributes, eg. to be able to tell why the message couldn’t be delivered.

Note that multiple copies of the same message can be sent to the error sink due to multiple subscription failures.

Brokers might decide to change the event type before reposting the failed event into the broker. This could be done by having a special error sink specific to broker.

### CloudEvent extensions

(might move to the CloudEvent spec repository)

Here a possible set of CloudEvent extensions:

* errorsubscriberuri: The URI of the subscriber
* errorreason: The reason for dead lettering the event
* errorretry: How many times the channel tried to send the message