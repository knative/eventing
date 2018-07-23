# Kafka - Knative Bus

Deployment steps:
1. Setup [Knative Eventing](../../../DEVELOPMENT.md)
1. Install a Kafka broker. A simple setup is provided:
    ```
    kubectl create namespace kafka
    kubectl apply -n kafka -f config/buses/kafka/broker/kafka-broker.yaml
    ```
1. Configure the bus to use the Kafka broker, replace the broker URL if not using the provided broker:
    ```
    kubectl create configmap kafka-bus-config --from-literal=KAFKA_BROKERS=kafkabroker.kafka:9092
    ```
1. For cluster wide deployment, change the kind in `config/buses/kafka/kafka-bus.yaml` from `Bus` to `ClusterBus`.
1. Apply the Kafka Bus:
    ```
    ko apply -f config/buses/kafka/
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
- for the dispatcher `kail -d kafka-bus -c dispatcher`
- for the provisioner `kail -d kafka-bus-provisioner -c provisioner`