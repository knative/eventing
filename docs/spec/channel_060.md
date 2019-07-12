# DEPRECATED Channel Spec (version 0.6.0)

## Background

The current system consists of two major APIs, the `Channel` and their
`ClusterChannelProvisioner`. Both are living in the
`eventing.knative.dev/v1alpha1` API Group.

### The Channel

A `kind: Channel` logically receives events on its input domain and forwards
them to its subscribers. Below is a specification for the different parts of the
`kind: Channel`.

### The ClusterChannelProvisioner

Describes an abstract configuration of a Source system which produces events or
a Channel system that receives and delivers events.

## Channel Spec Parts

### Control Plane

There exists a single Channel CRD. Users and the defaulting webhook are allowed
to specify `spec.provisioner`. The only programmatic interactions made with the
Channel are via the Channelable duck type, which revolves around the injection
of `spec.subscribers`, and its `status.conditions[?(.type == "Ready")].status`.

#### Default Channel setup

The notion of the used default channel is configured in the
`default-channel-webhook` Config map, and can be changed to a given
`ClusterChannelProvisioner`:

```
kind: ConfigMap
metadata:
  name: default-channel-webhook
  namespace: knative-eventing
data:
  default-channel-config: |
    clusterdefault:
      apiversion: eventing.knative.dev/v1alpha1
      kind: ClusterChannelProvisioner
      name: in-memory
    namespacedefaults:
      some-namespace:
        apiversion: eventing.knative.dev/v1alpha1
        kind: ClusterChannelProvisioner
        name: some-other-provisioner
```

Example of a channel, backed by the default config (`in-memory` CCP by default):

```
apiVersion: eventing.knative.dev/v1alpha1
kind: Channel
metadata:
  name: my-channel
```

#### Setup of a non-default Channel

With the explicit reference to a `spec.provisioner`, a channel is backed by its
available `ClusterChannelProvisioner`. Below is an example for a _Kafka
Channel_:

```
apiVersion: eventing.knative.dev/v1alpha1
kind: Channel
metadata:
  name: my-kafka-channel
spec:
  provisioner:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: ClusterChannelProvisioner
    name: kafka
```

#### Broker and Triggers

With the usage of the `knative-eventing-injection: enabled` label a default
broker is configured, which uses the default Channel's CCP:

```
apiVersion: eventing.knative.dev/v1alpha1
kind: Broker
metadata:
  name: default
```

##### Overriding the channel of a Broker

With the usage of the `spec.channelTemplate`, a broker instance can change its
backing Channel:

```
apiVersion: eventing.knative.dev/v1alpha1
kind: Broker
metadata:
  name: default
spec:
  channelTemplate:
    provisioner:
      apiVersion: eventing.knative.dev/v1alpha1
      kind: ClusterChannelProvisioner
      name: gcp-pubsub
```

Details about Broker and Trigger can be found [here](../broker/README.md).

### Data Plane

The data plane describes the input and output flow of a _Channel_:

#### Input

The input container must have an HTTP endpoint running on port 80. However, it
might be possible to run on a non-standard port, depending on how well calling
pieces handle the Addressable contract. It might be possible to support HTTPS.
The HTTP request MUST be a POST to the path '/', where anything else is rejected
with a `405 Method Not Supported` and `404 Not Found` respectively.

The HTTP body is passed through and all HTTP headers that match an allowed list
(e.g. x-b3-\*) are passed through (TODO: document them). It is used exclusively
in CloudEvents 0.2 HTTP binary content mode.

Currently there is no request validation is performed. For example:

- All conforming CloudEvents must specify a Type, but a Channel will happily
  pass along a request that does not have a Type.
- Channels will send a body of size zero and no headers, essentially a totally
  blank message.

HTTP status codes that are used:

- `202 Accepted` - The request was received and sent to the middleware
  successfully.
- `404 Not Found`:
  - The request was sent to path other than '/'.
  - The request was sent to a Channel that the dispatcher did not know about.
- `405 Method Not Supported` - The request was not a POST.
- `500 Server Error` - All other errors. Such as:

  - Could not extract the Channel name from the host.
  - Could not read the entire HTTP body.
  - Problem sending to the middleware.

  _NOTE: _ There is no authentication or authorization performed. If a request
  is received, then it is allowed.

#### Output

An HTTP POST is made to the URI that is injected into the Channel's spec field.
It is normally just HTTP, to a hostname, with a path of '/', because that is
what Addressable currently supports. It can be an arbitrary URI. However, it
almost certainly needs to be HTTP(S), due to the usage of the `http.Client`
package in current implementations.

The request is made by taking the original HTTP body combining it with all the
passed through headers. The request is made effectively as a CloudEvents 0.2
HTTP binary content mode request.

Different Channels handle failures in different ways:

- gcp-pubsub - Exponential backoff, up to five minutes. Will continue retrying
  forever.
- kafka - Immediate retry, no backoff. Will continue retrying forever.
  - Due to head of line blocking, nothing else get through this topic's
    partition.
- natss - Immediate retry, no backoff. Will continue retrying forever.
- in-memory - No retries. An error message is logged and the event is discarded.
