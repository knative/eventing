# Apache Kafka - Knative Bus

Deployment steps:
1. Setup [Knative Eventing](../../../DEVELOPMENT.md)
1. Install an Apache Kafka cluster. There are two choices:
   * Simple installation of [Apache Kafka](broker).
   * A production grade installation using the [Strimzi Kafka Operator](strimzi).

1. Now that the Apache Kafka is installed, configure the bus to use the Kafka broker, replace the broker URL if not using the provided broker:
    ```
    kubectl -n knative-eventing create configmap kafka-bus-config --from-literal=KAFKA_BOOTSTRAP_SERVERS=kafkabroker.kafka:9092
    ```
    > Note: If you are using Strimzi, the value for the URL is `my-cluster-kafka-bootstrap.kafka.9092`.
1. For cluster wide deployment, change the kind in `config/buses/kafka/kafka-bus.yaml` from `Bus` to `ClusterBus`.
1. Apply the Kafka Bus:
    ```
    ko apply -f config/buses/kafka/
    ```
1. If you want to set the default Knative Bus to Kafka run the following command to edit the Knative Eventing configuration (requires the above change in kind from `Bus` to `ClusterBus`):
    ```shell
    kubectl get cm flow-controller-config -n knative-eventing -oyaml  \
    | sed -e 's/default-cluster-bus: stub/  default-cluster-bus: kafka/' \
    | kubectl replace -f -
    ```
1. Create Channels that reference the 'kafka' Bus
1. (Optional) Install [Kail](https://github.com/boz/kail) - Kubernetes tail

The bus has an independent provisioner and dispatcher.

The provisioner will create Kafka topics for each Knative Channel
targeting the Bus (named `<namespace>.<channel-name>`.
Clients should avoid interacting with topics provisioned by the bus.

The dispatcher
- receives events via a Channel's Service from inside the cluster and
writes them to the corresponding Kafka topic
- creates a Kafka consumer for each `Subscription`, that reads events
from the subscription's channel and forwards them over HTTP to the
subscriber.

To view logs:
- for the dispatcher `kail -d kafka-[namespace]-bus-dispatcher -c dispatcher`
- for the provisioner `kail -d kafka-[namespace]-bus-provisioner -c provisioner`
