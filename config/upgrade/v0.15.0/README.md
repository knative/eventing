# Upgrade script (optional) to upgrade to v0.15.0 of Eventing

This directory contains a job that updates all of v1alpha1 based Brokers that
are using Spec.ChannelTemplate to specify which channel they use. You are not
able to create new ones specifying the ChannelTemplate, and you can either
manually update them yourself, or run the tool provided here, which will do the
following for any Broker that is using Spec.ChannelTemplate:

1. Creates a ConfigMap in the same namespace as the Broker named:
   `broker-upgrade-auto-gen-config-<brokername>` with the content from
   Spec.ChannelTemplate.
1. Set Broker Spec.Config to point to this ConfigMap
1. Set Broker Spec.ChannelTemplate to nil

To run the upgrade script:

```shell
kubectl apply -f https://github.com/knative/eventing/releases/download/v0.15.0/upgrade-to-v0.15.0.yaml
```

It will create a job called v0.15.0-upgrade in the knative-eventing namespace.
If you installed to different namespace, you need to modify the upgrade.yaml
appropriately. Also the job by default runs as `eventing-controller` service
account, you can also modify that but the service account will need to have
permissions to list `Namespace`s, list and patch `Broker`s, and create
`ConfigMap`s.

# Examples

If for example you have an existing v1lpha1 Broker with the following Spec:

```yaml
spec:
  channelTemplateSpec:
    apiVersion: messaging.knative.dev/v1alpha1
    kind: InMemoryChannel
```

The tool will create a ConfigMap that looks like so:

```yaml
apiVersion: v1
data:
  channelTemplateSpec: |2

    apiVersion: "messaging.knative.dev/v1alpha1"
    kind: "InMemoryChannel"
kind: ConfigMap
metadata:
  name: broker-upgrade-auto-gen-config-newbroker
  namespace: test-broker-6
  ownerReferences:
  - apiVersion: eventing.knative.dev/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Broker
    name: newbroker
```

And your Broker will then look like this after the upgrade:

```yaml
spec:
  config:
    apiVersion: v1
    kind: ConfigMap
    name: broker-upgrade-auto-gen-config-newbroker
    namespace: test-broker-6
```

For KafkaChannels it might look like something like this:

```yaml
spec:
  channelTemplateSpec:
    apiVersion: messaging.knative.dev/v1alpha1
    kind: KafkaChannel
    spec:
      numPartitions: 1
      replicationFactor: 1
```

The resulting ConfigMap will be:

```yaml
apiVersion: v1
data:
  channelTemplateSpec: |2

    apiVersion: "messaging.knative.dev/v1alpha1"
    kind: "KafkaChannel"
    spec:
      numPartitions: 1
      replicationFactor: 1
kind: ConfigMap
metadata:
  name: broker-upgrade-auto-gen-config-newbroker-kafka
  namespace: test-broker-6
  ownerReferences:
  - apiVersion: eventing.knative.dev/v1alpha1
    blockOwnerDeletion: true
    controller: true
    kind: Broker
    name: newbroker-kafka
```

And the Broker will look like this:

```yaml
spec:
  config:
    apiVersion: v1
    kind: ConfigMap
    name: broker-upgrade-auto-gen-config-newbroker-kafka
    namespace: test-broker-6
```
