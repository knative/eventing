# Apache Kafka Channels

Kafka channels are those backed by [Apache Kafka](http://kafka.apache.org/) topics.

## Deployment steps

1. Setup [Knative Eventing](../../../DEVELOPMENT.md)
1. If not done already, install an Apache Kafka cluster!

   - For Kubernetes a simple installation is done using the
     [Strimzi Kafka Operator](http://strimzi.io). Its installation
     [guides](http://strimzi.io/quickstarts/) provide content for Kubernetes and
     Openshift.

   > Note: This _Channel_ is not limited to Apache Kafka installations on
   > Kubernetes. It is also possible to use an off-cluster Apache Kafka
   > installation.

1. Now that Apache Kafka is installed, you need to configure the
   `bootstrap_servers` value in the `config-kafka` ConfigMap,
   located inside the `contrib/kafka/config/400-kafka-config.yaml` file:

    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: config-kafka
      namespace: knative-eventing
    data:
      # Broker URL. Replace this with the URLs for your kafka cluster,
      # which is in the format of my-cluster-kafka-bootstrap.my-kafka-namespace:9092.
      bootstrap_servers: REPLACE_WITH_CLUSTER_URL
    ```

1. Apply the Kafka config:

   ```
   ko apply -f contrib/kafka/config
   ```

1. Create the kafka channel custom objects:

   ```yaml
    apiVersion: messaging.knative.dev/v1alpha1
    kind: KafkaChannel
    metadata:
      name: my-kafka-channel
    spec:
      numPartitions: 1
      replicationFactor: 3
    ```
    You can configure the number of partitions with `numPartitions`, as well as the replication factor with `replicationFactor`. If not set, both will default to `1`.

## Components

The major components are:

- Kafka Channel Controller
- Kafka Channel Dispatcher
- Kafka Webhook
- Kafka Config Map


The Kafka Channel Controller is located in one Pod:

```shell
kubectl get deployment -n knative-eventing kafka-ch-controller
```

The Kafka Channel Dispatcher receives and distributes all events to the appropriate consumers:

```shell
kubectl get deployment -n knative-eventing kafka-ch-dispatcher
```

The Kafka Webhook is used to validate and set defaults to `KafkaChannel` custom objects:

```shell
kubectl get deployment -n knative-eventing kafka-webhook
```

The Kafka Config Map is used to configure the `bootstrap_servers` of your Apache Kafka installation:

```shell
kubectl get configmap -n knative-eventing config-kafka
```
