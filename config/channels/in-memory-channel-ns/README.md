# Namespace-Scoped In-Memory Channels

Namespace-scoped in-memory channels share the same characteristics as the
[in-memory channels](../in-memory-channel/README.md). The only difference is the
in-memory dispatcher is installed in the same namespace as the channel.

### Deployment steps:

1. Setup [Knative Eventing](../../../DEVELOPMENT.md).
1. Apply the `InMemoryChannel` CRD and controller.
   ```shell
   ko apply -f config/channels/in-memory-channel-ns
   ```
1. Create InMemoryChannels

   ```sh
   kubectl apply --filename - << END
   apiVersion: messaging.knative.dev/v1alpha1
   kind: InMemoryChannel
   metadata:
     name: foo
   END
   ```

IMPORTANT: make sure you don't have the
[cluster-scoped in-memory channels](../in-memory-channel/README.md)
configuration deployed in your cluster. Pick one or the other, but not both at
the same time!

### Components

The major components are:

- InMemoryChannel Controller
- InMemoryChannel Dispatcher

```shell
kubectl get deployment -n knative-eventing imc-controller
```

The InMemoryChannel Dispatcher receives and distributes all events. There is one
Dispatcher per namespace.

```shell
kubectl get deployment -n default imc-dispatcher
```

### Cleanup

To remove the in-memory channel component, do:

```shell
kubectl delete -f config/channels/in-memory-channel-ns
```

To remove the InMemoryChannel Dispatcher deployments, do:

```shell
kubectl delete deployments imc-dispatcher
```

in all namespaces you installed channels.
