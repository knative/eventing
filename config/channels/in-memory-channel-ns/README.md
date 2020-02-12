# Namespace-Scoped In-Memory Channels

Namespace-scoped in-memory channels share the same characteristics as the
[in-memory channels](../README.md). The only difference is the in-memory
dispatcher is installed in the same namespace as the channel.

### Deployment steps:

1. Setup [Knative Eventing](../../../DEVELOPMENT.md).
1. Apply the `InMemoryChannel` CRD and controller.
   ```shell
   ko apply -f config/channels/in-memory-channel-ns
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

The InMemoryChannel Dispatcher receives and distributes all events. There is
one Dispatcher per namespace.

```shell
kubectl get deployment -n default imc-dispatcher
```
