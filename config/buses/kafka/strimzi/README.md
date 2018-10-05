# Strimzi - Apache Kafka Operator

[Strimzi](http://strimzi.io) makes it easy to run a production grade Apache Kafka installation on OpenShift or Kubernetes. It implements the _Kubernetes Operator pattern_ for mananging `clusters`, `topics` or `users` based on custom resource files. 

Installing the Strimzi Cluster Operator is simple and requires only a few steps.

1. Create the `kafka` namespace in your Kubernetes cluster:
    ```
    kubectl create namespace kafka
    ```

1. Install the Strimzi _Cluster Operator_:

    * Applying yaml files from the [Strimzi release bundle](https://github.com/strimzi/strimzi-kafka-operator/releases/latest)
    * Using the Strimzi Helm Chart

    Both ways for installing the _Cluster Operator_ are described in the [Strimzi documentation](http://strimzi.io/docs/master/#cluster-operator-str) itself

    > Note: Once this is done, you will have a `strimzi-cluster-operator` pod, which is able to install the Apache Kafka broker based on a `Kafka` custom resource file.

1. Install the Apache Kafka cluster by providing the `kafka-persistent.yaml` Strimzi resource file from _this_ folder:
      ```
      kubectl apply -f kafka-persistent.yaml -n kafka
      ```
    > Note: If you want to use ephemeral storage, you have to use the `kafka-ephemeral.yaml` file.

    This provisions the complete installation of your Apache Kafka cluster.

> Note: For learning more about Strimiz, please consult its [website](http://strimzi.io).

Continue the configuration of Knative Eventing with [step `3`](../).
