# In-Memory Channels

In-memory channels are a best effort Channel. They should **NOT** be used in Production. They are
useful for development.

They differ from most Channels in that they have:

- No persistence.
  - If a Pod goes down, messages go with it.
- No ordering guarantee.
  - There is nothing enforcing an ordering, so two messages that arrive at the same time may
    go downstream in any order.
  - Different downstream subscribers may see different orders.
- No redelivery attempts.
  - If downstream rejects a request, a log message is written, but that request is never sent
    again.

### Deployment steps:

1. Setup [Knative Eventing](../../../DEVELOPMENT.md).
1. Apply the 'in-memory-channel' ClusterChannelProvisioner, Controller, and Dispatcher.
   ```shell
   ko apply -f config/provisioners/in-memory-channel/in-memory-channel.yaml
   ```
1. Create Channels that reference the 'in-memory-channel'.

   ```yaml
   apiVersion: eventing.knative.dev/v1alpha1
   kind: Channel
   metadata:
     name: foo
   spec:
     provisioner:
       apiVersion: eventing.knative.dev/v1alpha1
       kind: ClusterChannelProvisioner
       name: in-memory-channel
   ```

### Components

The major components are:

- ClusterChannelProvisioner Controller
- Channel Controller
- Channel Dispatcher
- Channel Dispatcher Config Map.

The ClusterChannelProvisioner Controller and the Channel Controller are colocated in one Pod.

```shell
kubectl get deployment -n knative-eventing in-memory-channel-controller
```

The Channel Dispatcher receives and distributes all events. There is a single Dispatcher for all
in-memory Channels.

```shell
kubectl get deployment -n knative-eventing in-memory-channel-dispatcher
```

The Channel Dispatcher Config Map is used to send information about Channels and Subscriptions from
the Channel Controller to the Channel Dispatcher.

```shell
kubectl get configmap -n knative-eventing in-memory-channel-dispatcher-config-map
```
