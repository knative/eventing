# Broker and Trigger CRDs

The Broker and Trigger CRDs, both in `eventing.knative.dev/v1alpha1`, are
interdependent.

## Broker

Broker represents an 'event mesh'. Events are sent to the Broker's ingress and
are then sent to any subscribers that are interested in that event. Once inside
a Broker, all metadata other than the CloudEvent is stripped away (e.g. unless
set as a CloudEvent attribute, there is no concept of how this event entered the
Broker).

Example:

```yaml
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

## Trigger

Trigger represents a desire to subscribe to events from a specific Broker. Basic
filtering on the types of events is provided.

Example:

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Trigger
metadata:
  name: my-service-trigger
spec:
  filter:
    sourceAndType:
      type: dev.knative.foo.bar
  subscriber:
    ref:
      apiVersion: serving.knative.dev/v1alpha1
      kind: Service
      name: my-service
```

## Usage

### Broker

There are two ways to create a Broker, via [namespace annotation](#annotation)
or [manual setup](#manual-setup).

Normally the [namespace annotation](#annotation) is used to do this setup.

#### Annotation

The easiest way to get started, is to annotate your namespace (replace `default`
with the desired namespace):

```shell
kubectl label namespace default knative-eventing-injection=enabled
```

This should automatically create the `default` `Broker` in that namespace.

```shell
kubectl -n default get broker default
```

#### Manual Setup

In order to setup a `Broker` manually, we must first create the required
`ServiceAccount` and give it the proper RBAC permissions. This setup is required
once per namespace. These instructions will use the `default` namespace, but you
can replace it with any namespace you want to install a `Broker` into.

Create the `ServiceAccount`.

```shell
kubectl -n default create serviceaccount eventing-broker-filter
```

Then give it the needed RBAC permissions:

```shell
kubectl -n default create rolebinding eventing-broker-filter \
  --clusterrole=eventing-broker-filter \
  --user=eventing-broker-filter
```

Note that the previous commands uses three different objects, all named
`eventing-broker-filter`. The `ClusterRole` is installed with Knative Eventing
[here](../../config/200-broker-clusterrole.yaml). The `ServiceAccount` was
created two commands prior. The `RoleBinding` is created with this command.

Now we can create the `Broker`. Note that this example uses the name `default`,
but could be replaced by any other valid name.

```shell
cat << EOF | kubectl apply -f -
apiVersion: eventing.knative.dev/v1alpha1
kind: Broker
metadata:
  namespace: default
  name: default
EOF
```

### Subscriber

Now create some function that wants to receive those events. This document will
assume the following, but it could be anything that is `Addressable`.

```yaml
apiVersion: serving.knative.dev/v1alpha1
kind: Service
metadata:
  name: my-service
  namespace: default
spec:
  runLatest:
    configuration:
      revisionTemplate:
        spec:
          container:
            # This corresponds to
            # https://github.com/knative/eventing-sources/blob/v0.2.1/cmd/message_dumper/dumper.go.
            image: gcr.io/knative-releases/github.com/knative/eventing-sources/cmd/message_dumper@sha256:ab5391755f11a5821e7263686564b3c3cd5348522f5b31509963afb269ddcd63
```

### Trigger

Create a `Trigger` that sends only events of a particular type to `my-service`:

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Trigger
metadata:
  name: my-service-trigger
  namespace: default
spec:
  filter:
    sourceAndType:
      type: dev.knative.foo.bar
  subscriber:
    ref:
      apiVersion: serving.knative.dev/v1alpha1
      kind: Service
      name: my-service
```

#### Defaulting

The Webhook will default certain unspecified fields. For example if
`spec.broker` is unspecified, it will default to `default`. If
`spec.filter.sourceAndType.type` or `spec.filter.sourceAndType.Source` are
unspecified, then they will default to the special value `Any`, which matches
everything.

The Webhook will default the YAML above to:

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Trigger
metadata:
  name: my-service-trigger
  namespace: default
spec:
  broker: default # Defaulted by the Webhook.
  filter:
    sourceAndType:
      type: dev.knative.foo.bar
      source: Any # Defaulted by the Webhook.
  subscriber:
    ref:
      apiVersion: serving.knative.dev/v1alpha1
      kind: Service
      name: my-service
```

You can make multiple `Trigger`s on the same `Broker` corresponding to different
types, sources, and subscribers.

### Source

Now have something emit an event of the correct type (`dev.knative.foo.bar`)
into the `Broker`. We can either do this manually or with a normal Knative
Source.

#### Manual

The `Broker`'s address is well known, it will always be
`<name>-broker.<namespace>.svc.<ending>`. In our case, it is
`default-broker.default.svc.cluster.local`.

While SSHed into a `Pod` with the Istio sidecar, run:

```shell
curl -v "http://default-broker.default.svc.cluster.local/" \
  -X POST \
  -H "X-B3-Flags: 1" \
  -H "CE-CloudEventsVersion: 0.1" \
  -H "CE-EventType: dev.knative.foo.bar" \
  -H "CE-EventTime: 2018-04-05T03:56:24Z" \
  -H "CE-EventID: 45a8b444-3213-4758-be3f-540bf93f85ff" \
  -H "CE-Source: dev.knative.example" \
  -H 'Content-Type: application/json' \
  -d '{ "much": "wow" }'
```

#### Knative Source

Provide the Knative Source the `default` `Broker` as its sink:

```yaml
apiVersion: sources.eventing.knative.dev/v1alpha1
kind: ContainerSource
metadata:
  name: heartbeats-sender
spec:
  image: github.com/knative/eventing-sources/cmd/heartbeats/
  sink:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Broker
    name: default
```

## Implementation

Broker and Trigger are intended to be black boxes. How they are implemented
should not matter to the end user. This section describes the specific
implementation that is currently in the repository. However, **the implmentation
may change at any time, absolutely no guarantees are made about the
implmentation**.

### Namespace

Namespaces are reconciled by the
[Namespace Reconciler](../../pkg/reconciler/v1alpha1/namespace). The
`Namespace Reconciler` looks for all `namespace`s that have the label
`knative-eventing-injection: enabled`. If that label is present, then the
`Namespace Reconciler` reconciles:

1. Creates the Broker Filter's `ServiceAccount`, `eventing-broker-filter`.
1. Ensures that `ServiceAccount` has the requisite RBAC permissions by giving it
   the [`eventing-broker-filter`](../../config/200-broker-clusterrole.yaml)
   `Role`.
1. Creates a `Broker` named `default`.

### Broker

`Broker`s are reconciled by the
[Broker Reconciler](../../pkg/reconciler/v1alpha1/broker). For each `Broker`, it
reconciles:

1. The 'trigger' `Channel`. This is a `Channel` that all events in the
   `Broker` are sent to. Anything that passes the `Broker`'s Ingress is sent to
   this `Channel`. All `Trigger`s subscribe to this `Channel`.
1. The 'filter' `Deployment`. The `Deployment` runs
   [cmd/broker/filter](../../cmd/broker/filter). Its purpose is the data plane
   for all `Trigger`s related to this `Broker`.
   - This piece is very similar to the existing Channel dispatchers, in that all
     `Trigger`s for a given `Broker` route to this single `Deployment`. The code
     inspects the Host header to determine which `Trigger` the request is
     related to and then carries it out.
   - Internally this binary uses the [pkg/broker](../../pkg/broker) library.
1. The 'filter' Kubernetes `Service`. This `Service` points to the 'filter'
   `Deployment`.
1. The 'ingress' `Deployment`. The `Deployment` runs
   [cmd/broker/ingress](../../cmd/broker/ingress). Its purpose is to inspect all
   events that are entering the `Broker`.
1. The 'ingress' Kubernetes `Service`. This `Service` points to the 'ingress'
   `Deployment`. This `Service`'s address is the address given for the `Broker`.
1. The 'ingress' `Channel`. This is a `Channel` for replies from `Trigger`s.
   It should not be used by anything else.
1. The 'ingress' `Subscription` which subscribes from the 'ingress' `Channel` to
   the 'ingress' `Service`.

### Trigger

`Trigger`s are reconciled by the
[Trigger Reconciler](../../pkg/reconciler/v1alpha1/trigger). For each `Trigger`,
it reconciles:

1. Determines the subscriber's URI.
   - Currently uses the same logic as the `Subscription` Reconciler, so supports
     Addressables and Kubernetes `Service`s.
1. Creates a Kubernetes `Service` and Istio `VirtualService` pair. This allows
   all Istio enabled `Pod`s to send to the `Trigger`'s address.
   - This is the same as the current `Channel` implementation. The `Service`
     points nowhere. The `VirtualService` reroutes requests that originally went
     to the `Service`, to instead go to the `Broker`'s 'filter' `Service`.
1. Creates `Subscription` from the `Broker`'s 'trigger' `Channel` to the
   `Trigger`'s Kubernetes `Service`. Replies are sent to the `Broker`'s
   'ingress' `Channel`.
