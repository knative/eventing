# Broker and Trigger CRDs

The `Broker` and `Trigger` CRDs are documented in the
[docs repo](https://knative.dev/docs/eventing/).

## Implementations

How Broker and Trigger are implemented should not matter to the end user. This
section describes the specific implementations that are currently in the
repository. However, **the implementation may change at any time, absolutely no
guarantees are made about the implementation**.

### Multi Tenant Channel Based Broker

#### [High level implementation details](https://docs.google.com/document/d/1yUrm9IsgwIfSQfeg0hcUgNFN0RvBfxEv1-jYoeyPigE/edit)

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
[Broker Reconciler](../../pkg/reconciler/broker/broker.go). For each `Broker`,
it reconciles:

1. The 'trigger' `Channel`. This is a `Channel` that all events received by
   broker-ingress targeting this broker are sent to.

#### Trigger

`Trigger`s are reconciled by the Broker that the triggers belong to. Each
individual Trigger is reconciled using
[Trigger Reconciler](../../pkg/reconciler/broker/trigger/trigger.go). For each
`Trigger`, it reconciles:

1. Check and propagate Broker status
1. Determine the Subscriber's URI
   - Currently uses the same logic as the `Subscription` Reconciler, so supports
     Addressables and Kubernetes `Service`s.
1. Creates a `Subscription` from the `Broker`'s 'trigger' `Channel` to the
   broker-filter service using the HTTP path `/triggers/{namespace}/{name}`.
   Replies are sent to the broker-ingress/namespace/broker
