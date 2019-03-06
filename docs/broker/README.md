## Broker and Trigger CRDs

The Broker and Trigger CRDs, both in `eventing.knative.dev/v1alpha1`, are
interdependent.

#### Broker

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

#### Trigger

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

The easiest way to get started, is to annotate your namespace (replace `default` with the desired namespace):

```shell
kubectl label namespace default knative-eventing-injection=enabled
```

This should automatically create the `default` `Broker` in that namespace.

```shell
kubectl -n default get broker default
```

Now create some function that wants to receive those events. 

## TODO

Now create any triggers against that broker:

## TODO

### Implementation

Broker and Trigger are intended to be black boxes. How they are implemented
should not matter to the end user. This section describes the specific
implementation that is currently in the repository. However, **the implmentation
may change at any time, absolutely no guarantees are made about the
implmentation**.

#### Namespace

Namespaces are reconciled by the
[Namespace Reconciler](../../pkg/reconciler/v1alpha1/namespace). The `Namespace
Reconciler` looks for all `namespace`s that have the label
`knative-eventing-injection: enabled`. If that label is present, then the
`Namespace Reconciler` reconciles:

1.  Creates the Broker Filter's `ServiceAccount`, `eventing-broker-filter`.
1.  Ensures that `ServiceAccount` has the requisite RBAC permissions by giving
    it the [`eventing-broker-filter`](../../config/200-broker-clusterrole.yaml)
    `Role`.
1.  Creates a `Broker` named `default`.

#### Broker

`Broker`s are reconciled by the
[Broker Reconciler](../../pkg/reconciler/v1alpha1/broker). For each `Broker`, it
reconciles:

1.  The 'everything' `Channel`. This is a `Channel` that all events in the
    `Broker` are sent to. Anything that passes the `Broker`'s Ingress is sent to
    this `Channel`. All `Trigger`s subscribe to this `Channel`.
1.  The 'filter' `Deployment`. The `Deployment` runs
    [cmd/broker/filter](../../cmd/broker/filter). Its purpose is the data plane
    for all `Trigger`s related to this `Broker`.
    -   This piece is very similar to the existing Channel dispatchers, in that
        all `Trigger`s for a given `Broker` route to this single `Deployment`.
        The code inspects the Host header to determine which `Trigger` the
        request is related to and then carries it out.
    -   Internally this binary uses the [pkg/broker](../../pkg/broker) library.
1.  The 'filter' Kubernetes `Service`. This `Service` points to the 'filter'
    `Deployment`.
1.  The 'ingress' `Deployment`. The `Deployment` runs
    [cmd/broker/ingress](../../cmd/broker/ingress). Its purpose is to inspect
    all events that are entering the `Broker`.
1.  The 'ingress' Kubernetes `Service`. This `Service` points to the 'ingress'
    `Deployment`. This `Service`'s address is the address given for the
    `Broker`.

#### Trigger

`Trigger`s are reconciled by the
[Trigger Reconciler](../../pkg/reconciler/v1alpha1/trigger). For each `Trigger`,
it reconciles:

1.  Determines the subscriber's URI.
    -   Currently uses the same logic as the `Subscription` Reconciler, so
        supports Addressables and Kubernetes Services.
1.  Creates a Kubernetes `Service` and Istio `VirtualService` pair. This allows
    all Istio enabled `Pod`s to send to the `Trigger`'s address.
    -   This is the same as the current `Channel` implementation. The `Service`
        points no where. The `VirtualService` reroutes requests that originally
        went to the `Service`, to instead go to the `Broker`'s 'filter'
        `Service`.
1.  Creates `Subscription` from the `Broker`'s 'everything' `Channel` to the
    `Trigger`'s Kubernetes `Service`.
