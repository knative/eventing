# Broker and Trigger CRDs

The `Broker` and `Trigger` CRDs are documented in the
[docs repo](https://github.com/knative/docs/blob/master/docs/eventing/broker-trigger.md).

## Implementations

Broker and Trigger are intended to be black boxes. How they are implemented
should not matter to the end user. This section describes the specific
implementations that are currently in the repository. However, **the
implementation may change at any time, absolutely no guarantees are made about
the implementation**.

### Channel Based Broker

#### Installation

To install a Channel Based Broker, you must first install the Knative Eventing
core followed by the Broker Implementation:

```
ko apply -f config/brokers/channel-broker/
```

#### Uninstallation

To uninstall a Channel Based Broker.

```
kubectl delete -f config/brokers/channel-broker/
```

#### BrokerClass

To indicate that Channel Based Broker should be used to reconcile a Broker, you
have to use an annotation specifying Channel Based Broker as the Broker Class:

```
    annotations:
      eventing.knative.dev/broker.class: ChannelBasedBroker
```

#### Namespace

Namespaces are reconciled by the
[Namespace Reconciler](../../pkg/reconciler/namespace). The
`Namespace Reconciler` looks for all `namespace`s that have the label
`knative-eventing-injection: enabled`. If that label is present, then the
`Namespace Reconciler` reconciles:

1. Creates the Broker Ingress' `ServiceAccount`, `eventing-ingress-filter`.
1. Ensures that `ServiceAccount` has the requisite RBAC permissions by giving it
   the [`eventing-broker-ingress`](../../config/200-broker-clusterrole.yaml)
   `Role` and the
   [`eventing-config-reader`](../../config/200-broker-clusterrole.yaml) `Role`
   (in the system namespace).
1. Creates the Broker Filter's `ServiceAccount`, `eventing-broker-filter`.
1. Ensures that `ServiceAccount` has the requisite RBAC permissions by giving it
   the [`eventing-broker-filter`](../../config/200-broker-clusterrole.yaml)
   `Role` and the
   [`eventing-config-reader`](../../config/200-broker-clusterrole.yaml) `Role`
   (in the system namespace).
1. Creates a `Broker` named `default`.

#### Broker

`Broker`s are reconciled by the
[Broker Reconciler](../../pkg/reconciler/broker). For each `Broker`, it
reconciles:

1. The 'trigger' `Channel`. This is a `Channel` that all events in the `Broker`
   are sent to. Anything that passes the `Broker`'s Ingress is sent to this
   `Channel`. All `Trigger`s subscribe to this `Channel`.
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

#### Trigger

`Trigger`s are reconciled by the Broker that the triggers belong to. Each
individual Trigger is reconciled using
[Trigger Reconciler](../../pkg/reconciler/broker/trigger.go). For each
`Trigger`, it reconciles:

1. Check and propagate Broker status
1. Get the Broker's:
   - Trigger Channel
   - Ingress Channel
   - Filter Service
1. Determine the Subscriber's URI
   - Currently uses the same logic as the `Subscription` Reconciler, so supports
     Addressables and Kubernetes `Service`s.
1. Creates a `Subscription` from the `Broker`'s 'trigger' `Channel` to the
   `Trigger`'s Kubernetes `Service` using the HTTP path
   `/triggers/{namespace}/{name}`. Replies are sent to the `Broker`'s 'ingress'
   `Channel`.

### Multi Tenant Channel Based Broker

#### [High level implementation details](https://docs.google.com/document/d/1qYnmkRduWLUFQ3vEsaw7jU_mxS_nDHHkDkcGRf1_Fy4/edit)

#### Installation

To install a Multi Tenant Channel Based Broker, you must first install the
Knative Eventing core followed by the MT Broker Implementation:

```
ko apply -f config/brokers/mt-channel-broker/
```

#### Uninstallation

To uninstall a Multi Tenant Channel Based Broker.

```
kubectl delete -f config/brokers/mt-channel-broker/
```

#### BrokerClass

To indicate that Multi Tenant Channel Based Broker should be used to reconcile a
Broker, you have to use an annotation specifying Multi Tenant Channel Based
Broker as the Broker Class:

```
    annotations:
      eventing.knative.dev/broker.class: MTChannelBasedBroker
```

#### Broker

Broker has 2 shared services that are shared between each Broker of this Broker
Class. By default, they live in knative-eventing namespace and are named
`broker-ingress` and `broker-filter`. The routing for incoming events to
broker-ingress is based off the Path portion of the incoming request of the form
/namespace/broker. More details are in the document under High level
implementation details above.

`Broker`s are reconciled by the
[Broker Reconciler](../../pkg/reconciler/mtbroker). For each `Broker`, it
reconciles:

1. The 'trigger' `Channel`. This is a `Channel` that all events received by
   broker-ingress targeting this broker are sent to.

#### Trigger

`Trigger`s are reconciled by the Broker that the triggers belong to. Each
individual Trigger is reconciled using
[Trigger Reconciler](../../pkg/reconciler/mtbroker/trigger.go). For each
`Trigger`, it reconciles:

1. Check and propagate Broker status
1. Determine the Subscriber's URI
   - Currently uses the same logic as the `Subscription` Reconciler, so supports
     Addressables and Kubernetes `Service`s.
1. Creates a `Subscription` from the `Broker`'s 'trigger' `Channel` to the
   broker-filter service using the HTTP path `/triggers/{namespace}/{name}`.
   Replies are sent to the broker-ingress/namespace/broker
