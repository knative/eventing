# Strimzi - Apache Kafka Operator

[Strimzi](http://strimzi.io) makes it easy to run a production grade Apache Kafka installation on OpenShift or Kubernetes. It implements the _Kubernetes Operator pattern_ for mananging `clusters`, `topics` or `users` based on custom resource files. 

Installing the Strimzi Operator is simple and requires only a few steps:

1. Create the `kafka` namespace and apply the Strimzi Roles, CRDs and the deployment:
    ```
    kubectl create namespace kafka
    kubectl apply -f resources -n kafka
    ```
    This will create a deployment for the `strimzi-cluster-operator` pod, which is able to install the Apache Kafka broker, based on a `Kafka` custom resource file.

1. Install the cluster by providing the `kafka-persistent.yaml` custom resource file:
      ```
      kubectl apply -f kafka-persistent.yaml -n kafka
      ```
    > Note: If you want to use ephemeral storage, you have to use the `kafka-ephemeral.yaml` file.

    This provisions the complete installation of your Apache Kafka cluster.

To contine the configuration of Knative Eventing, contine with [step `3`](../).
