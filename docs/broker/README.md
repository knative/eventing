# Broker and Trigger CRDs

The `Broker` and `Trigger` CRDs are documented in the
[docs repo](https://github.com/knative/docs/blob/master/docs/eventing/broker-trigger.md).


## Implementation

Broker and Trigger are intended to be black boxes. How they are implemented
should not matter to the end user. This section describes the specific
implementation that is currently in the repository. However, **the
implementation may change at any time, absolutely no guarantees are made about
the implementation**.

### Namespace

Namespaces are reconciled by the
[Namespace Reconciler](../../pkg/reconciler/namespace). The
`Namespace Reconciler` looks for all `namespace`s that have the label
`knative-eventing-injection: enabled`. If that label is present, then the
`Namespace Reconciler` reconciles:

1. Creates the Broker Ingress' `ServiceAccount`, `eventing-ingress-filter`.
1. Ensures that `ServiceAccount` has the requisite RBAC permissions by giving it
   the [`eventing-broker-ingress`](../../config/200-broker-clusterrole.yaml)
   `Role`.
1. Creates the Broker Filter's `ServiceAccount`, `eventing-broker-filter`.
1. Ensures that `ServiceAccount` has the requisite RBAC permissions by giving it
   the [`eventing-broker-filter`](../../config/200-broker-clusterrole.yaml)
   `Role`.
1. Creates a `Broker` named `default`.

### Broker

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
1. The 'ingress' `Channel`. This is a `Channel` for replies from `Trigger`s. It
   should not be used by anything else.
1. The 'ingress' `Subscription` which subscribes from the 'ingress' `Channel` to
   the 'ingress' `Service`.

### Trigger

`Trigger`s are reconciled by the
[Trigger Reconciler](../../pkg/reconciler/trigger). For each `Trigger`, it
reconciles:

1. Verify the Broker Exists
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
