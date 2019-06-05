# NATS Streaming Channels

NATS Streaming channels are beta-quality Channels that are backed by
[NATS Streaming](https://github.com/nats-io/nats-streaming-server).

They offer:

- Persistence
  - If the Channel's Pod goes down, all events already ACKed by the Channel will
    persist and be retransmitted when the Pod restarts.
- Redelivery attempts
  - If downstream rejects an event, that request is attempted again. NOTE:
    downstream must successfully process the event within one minute or the
    delivery is assumed to have failed and will be reattempted.

They do not offer:

- Ordering guarantees
  - Events seen downstream may not occur in the same order they were inserted
    into the Channel.

## Deployment steps

1. Setup [Knative Eventing](../../../DEVELOPMENT.md).
1. If not done already, install a [NATS Streaming](./broker)
1. Apply the NATSS configuration:

   ```shell
   ko apply -f contrib/natss/config
   ```

1. Create NATSS channels:

   ```yaml
   apiVersion: messaging.knative.dev/v1alpha1
   kind: NatssChannel
   metadata:
     name: foo
   ```

## Components

The major components are:

- NATS Streaming
- NATSS Channel Controller
- NATSS Channel Dispatcher

The NATSS Channel Controller is located in one Pod.

```shell
kubectl get deployment -n knative-eventing natss-ch-controller
```

The NATSS Channel Dispatcher receives and distributes all events. There is a
single Dispatcher for all NATSS Channels.

```shell
kubectl get deployment -n knative-eventing natss-ch-dispatcher
```

By default the components are configured to connect to NATS at
`nats://nats-streaming.natss.svc:4222` with NATS Streaming cluster ID
`knative-nats-streaming`. This may be overridden by configuring both the
`natss-ch-controller` and `natss-ch-dispatcher` deployments with the following
environment variables:

```yaml
env:
  - name: DEFAULT_NATSS_URL
    value: nats://natss.custom-namespace.svc.cluster.local:4222
  - name: DEFAULT_CLUSTER_ID
    value: custom-cluster-id
  - name: SYSTEM_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
```
