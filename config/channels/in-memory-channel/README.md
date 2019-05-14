# In-Memory Channels

In-memory channels are a best effort Channel. They should **NOT** be used in
Production. They are useful for development.

They differ from most Channels in that they have:

- No persistence.
  - If a Pod goes down, messages go with it.
- No ordering guarantee.
  - There is nothing enforcing an ordering, so two messages that arrive at the
    same time may go downstream in any order.
  - Different downstream subscribers may see different orders.
- No redelivery attempts.
  - If downstream rejects a request, a log message is written, but that request
    is never sent again.

### Deployment steps:

1. Setup [Knative Eventing](../../../DEVELOPMENT.md).
1. Apply the `InMemoryChannel` CRD, Controller, and Dispatcher.
   ```shell
   ko apply -f config/channels/in-memory-channel/...
   ```
1. Create InMemoryChannels

   ```yaml
   apiVersion: messaging.knative.dev/v1alpha1
   kind: InMemoryChannel
   metadata:
     name: foo
   ```

### Components

The major components are:

- InMemoryChannel Controller
- InMemoryChannel Dispatcher


```shell
kubectl get deployment -n knative-eventing imc-controller
```

The InMemoryChannel Dispatcher receives and distributes all events. There is a single
Dispatcher for all in-memory Channels.

```shell
kubectl get deployment -n knative-eventing imc-dispatcher
```
