# In-Memory Channels

In-memory channels are a best effort channel. They have the following
characterics:

- **No Persistence**.
  - When a Pod goes down, messages go with it.
- **No Ordering Guarantee**.
  - There is nothing enforcing an ordering, so two messages that arrive at the
    same time may go to subscribers in any order.
  - Different downstream subscribers may see different orders.
- **No Redelivery Attempts**.
  - When a subscriber rejects a message, there is no attempts to retry sending
    it.
- **Dead Letter Sink**.
  - When a subscriber rejects a message, this message is sent to the dead letter
    sink, if present, otherwise it is dropped.

### Deployment steps:

1. Setup [Knative Eventing](../../../DEVELOPMENT.md).
1. Apply the `InMemoryChannel` CRD, Controller, and cluster-scoped Dispatcher.
   ```shell
   ko apply -f config/channels/in-memory-channel/
   ```
1. Create InMemoryChannels

   ```shell
   kubectl apply --filename - << END
   apiVersion: messaging.knative.dev/v1alpha1
   kind: InMemoryChannel
   metadata:
     name: foo
   END
   ```

### Components

The major components are:

- InMemoryChannel Controller
- InMemoryChannel Dispatcher

```shell
kubectl get deployment -n knative-eventing imc-controller
```

The InMemoryChannel Dispatcher receives and distributes all events. There is a
single Dispatcher for all in-memory Channels.

```shell
kubectl get deployment -n knative-eventing imc-dispatcher
```

### Namespace Dispatchers

By default events are received and dispatched by a single cluster-scoped
dispatcher components. You can also specify whether events should be received
and dispatched by the dispatcher in the same namespace as the channel definition
by adding the `eventing.knative.dev/scope: namespace` annotation. For instance:

```shell
kubectl apply --filename - << END
apiVersion: messaging.knative.dev/v1alpha1
kind: InMemoryChannel
metadata:
  name: foo-ns
  namespace: default
  annotations:
    eventing.knative.dev/scope: namespace
END
```
