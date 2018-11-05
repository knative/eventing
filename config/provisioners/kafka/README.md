# Apache Kafka Channels

Deployment steps:
1. Setup [Knative Eventing](../../../DEVELOPMENT.md)
1. Install an Apache Kafka cluster. There are two choices:
   * Simple installation of [Apache Kafka](broker).
   * A production grade installation using the [Strimzi Kafka Operator](strimzi).

1. Now that the Apache Kafka is installed, apply the 'Kafka' ClusterChannelProvisioner:
    ```
    ko apply -f config/provisioners/kafka/kafka-provisioner.yaml
    ```
    > Note: If you are using Strimzi, you need to update the `KAFKA_BOOTSTRAP_SERVERS` value in the `kafka-channel-controller-config` ConfigMap to `my-cluster-kafka-bootstrap.kafka.9092`.
1. Create Channels that reference the 'kafka-channel'.

    ```yaml
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Channel
    metadata:
      name: my-kafka-channel
    spec:
      provisioner:
        apiVersion: eventing.knative.dev/v1alpha1
        kind: ClusterChannelProvisioner
        name: kafka-channel
    ```
1. (Optional) Install [Kail](https://github.com/boz/kail) - Kubernetes tail

## Components

The major components are:
* ClusterChannelProvisioner Controller
* Channel Controller Config Map

The ClusterChannelProvisioner Controller and the Channel Controller are colocated in one Pod.
```shell
kubectl get deployment -n knative-eventing kafka-channel-controller
```
