# Apache Kakfa - simple installation

1. For an installation of a simple Apache Kafka cluster, a setup is provided:
    ```
    kubectl create namespace kafka
    kubectl apply -n kafka -f kafka-broker.yaml
    ```
    > Note: If you are running Knative on OpenShift you will need to run the following command first to allow the Kafka broker to run as root:
      ```
      oc adm policy add-scc-to-user anyuid -z default -n kafka
      ```

Continue the configuration of Knative Eventing with [step `3`](../).
