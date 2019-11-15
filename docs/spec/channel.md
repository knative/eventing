# Channel Spec (IN PROGRESS)

## Background

Starting with Version 0.7.0 all the different Channel CRDs (e.g.
`InMemoryChannel` or `KafkaChannel`) are living in the
`messaging.knative.dev/v1alpha1` API Group.

A channel logically receives events on its input domain and forwards them to its
subscribers. Below is a specification for the generic parts of each _Channel_.

A typical channel consists of a _Controller_ and a _Dispatcher_ pod.

## Conformance

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "MAY", and "OPTIONAL" in this document are to be
interpreted as described in [RFC2119](https://www.ietf.org/rfc/rfc2119.txt).

## Channel Spec Parts

### API Group

The CRD's API group MAY be any valid API Group. If it is in `knative.dev`, then
it SHOULD be `messaging.knative.dev`.

### Kind Naming

The CRD's Kind SHOULD have the suffix `Channel`. The name MAY be just `Channel`.

### Control Plane

Each Channel implementation is backed by its own CRD (e.g. `InMemoryChannel` or
`KafkaChannel`). Below is an example for a `KafkaChannel` object:

```
apiVersion: messaging.knative.dev/v1alpha1
kind: KafkaChannel
metadata:
  name: kafka-channel
spec:
  numPartitions: 3
  replicationFactor: 1
```

A different example for the `InMemoryChannel`:

```
apiVersion: messaging.knative.dev/v1alpha1
kind: InMemoryChannel
metadata:
  name: my-channel
```

Each _Channel Controller_ ensures the required tasks on the backing technology
are applied. In this case a Kafka topic with the desired configuration is being
created, backing all messages from the channel.

#### Aggregated Channelable Manipulator ClusterRole

Every CRD MUST create a corresponding ClusterRole, that will be aggregated into
the `channelable-manipulator` ClusterRole. This ClusterRole MUST include
permissions to create, get, list, watch, patch, and update the CRD's custom
objects and their status. Below is an example for the `KafkaChannel`:

```
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kafka-channelable-manipulator
  labels:
    duck.knative.dev/channelable: "true"
# Do not use this role directly. These rules will be added to the "channelable-manipulator" role.
rules:
  - apiGroups:
      - messaging.knative.dev
    resources:
      - kafkachannels
      - kafkachannels/status
    verbs:
      - create
      - get
      - list
      - watch
      - update
      - patch
```

Each channel MUST have the `duck.knative.dev/channelable: "true"` label on its
`channelable-manipulator` ClusterRole.

#### Aggregated Addressable Resolver ClusterRole

Every CRD MUST create a corresponding ClusterRole, that will be aggregated into
the `addressable-resolver` ClusterRole. This ClusterRole MUST include
permissions to get, list, and watch the CRD's custom objects and their status.
Below is an example for the `InMemoryChannel`:

```
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: imc-addressable-resolver
  labels:
    eventing.knative.dev/release: devel
    duck.knative.dev/addressable: "true"
# Do not use this role directly. These rules will be added to the "addressable-resolver" role.
rules:
  - apiGroups:
      - messaging.knative.dev
    resources:
      - inmemorychannels
      - inmemorychannels/status
    verbs:
      - get
      - list
      - watch
```

Each channel MUST have the `duck.knative.dev/addressable: "true"` label on its
`addressable-resolver` ClusterRole.

#### CustomResourceDefinition per Channel

For each channel implementation a `CustomResourceDefinition` is created, like:

```
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
 name: kafkachannels.messaging.knative.dev
 labels:
    knative.dev/crd-install: "true"
    messaging.knative.dev/subscribable: "true"
spec:
  group: messaging.knative.dev
  version: v1alpha1
  names:
    kind: KafkaChannel
    plural: kafkachannels
    singular: kafkachannel
    categories:
    - all
    - knative
    - messaging
    - channel
    shortNames:
    - kc
  scope: Namespaced
...
```

Each channel is _namespaced_ and MUST have the following:

- label of `messaging.knative.dev/subscribable: "true"`
- The category `channel`.

### Data Plane

The data plane describes the input and output flow of a _Channel_. All Channels
exclusively communicate using CloudEvents.

#### Input

Every Channel MUST expose either an HTTP or HTTPS endpoint. It MAY expose both.
The endpoint(s) MUST conform to
[HTTP Transport Binding for CloudEvents - Version 0.3](https://github.com/cloudevents/spec/blob/v0.3/http-transport-binding.md).
It MUST support both Binary Content mode and Structured Content mode. The
HTTP(S) endpoint MAY be on any port, not just the standard 80 and 443. Channels
MAY expose other, non-HTTP endpoints in addition to HTTP at their discretion
(e.g. expose a gRPC endpoint to accept events).

##### Generic

If a Channel receives an event queueing request and is unable to parse a valid
CloudEvent, then it MUST reject the request.

The Channel MUST pass through all tracing information as CloudEvents attributes.
In particular, it MUST translate any incoming OpenTracing or B3 headers to the
[Distributed Tracing Extension](https://github.com/cloudevents/spec/blob/v0.3/extensions/distributed-tracing.md).
The Channel SHOULD sample and write traces to the location specified in
[`config-tracing`](https://github.com/cloudevents/spec/blob/v0.3/extensions/distributed-tracing.md).

##### HTTP

Channels MUST reject all HTTP event queueing requests with a method other than
POST responding with HTTP status code `405 Method Not Supported`. Non-event
queueing requests (e.g. health checks) are not constrained.

The HTTP event queueing request's URL MUST correspond to a single, unique
Channel at any given moment in time. This MAY be done via the host, path, query
string, or any combination of these. This mapping is handled exclusively by the
Channel implementation, exposed via the Channel's `status.address`. If an HTTP
event queueing request's URL does not correspond to an existing Channel, then
the Channel MUST respond with `404 Not Found`.

The Channel MUST respond with `202 Accepted` if the event queueing request is
accepted by the server.

If a Channel receives an event queueing request and is unable to parse a valid
CloudEvent, then it MUST respond with `400 Bad Request`.

#### Output

Channels MUST output CloudEvents. The output MUST be via a binding specified in
the
[CloudEvents specification](https://github.com/cloudevents/spec/tree/v0.3#cloudevents-documents).
Every Channel MUST support sending events via Binary Content Mode HTTP Transport
Binding.

Channels MUST NOT alter an event that goes through them. All CloudEvent
attributes, including the data attribute, MUST be received at the subscriber
identical to how they were received by the Channel. The only exception is the
[Distributed Tracing Extension Attribute](https://github.com/cloudevents/spec/blob/v0.3/extensions/distributed-tracing.md),
which is expected to change as the span id will be altered at every network hop.

##### Retries

Channels SHOULD retry resending CloudEvents when they fail to either connect or
send CloudEvents to subscribers.

Channels SHOULD support various retry configuration parameters, including, but
not limited to:

- the maximum number of retries
- the time in-between retries
- the backoff rate

#### Metrics

Channels SHOULD expose a variety of metrics, including, but not limited to:

- Number of malformed incoming event queueing events (`400 Bad Request`
  responses)
- Number of accepted incoming event queuing events (`202 Accepted` responses)
- Number of egress CloudEvents produced (with the former metric, used to derive
  channel queue size)

Metrics SHOULD be enabled by default, with a configuration parameter included to
disable them if desired.
