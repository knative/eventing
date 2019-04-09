# Apache Kafka Channels

Deployment steps:

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
   `bootstrap_servers` value in the `kafka-channel-controller-config` ConfigMap,
   located inside the `contrib/kafka/config/kafka.yaml` file:

   ```yaml
   ...
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: kafka-channel-controller-config
     namespace: knative-eventing
   data:
     # Broker URL's for the provisioner
     bootstrap_servers: my-cluster-kafka-bootstrap.my-kafka-namespace:9092

     # Consumer mode to dispatch events from different partitions in parallel.
     # By default(multiplex), partitions are multiplexed with a single go channel.
     # `multiplex` and `partitions` are valid values.
     ## consumer_mode: partitions
     ...
   ```

1. Apply the 'Kafka' ClusterChannelProvisioner, Controller, and Dispatcher:

   ```
   ko apply -f contrib/kafka/config/kafka.yaml
   ```

1. Create Channels that reference the 'kafka' ClusterChannelProvisioner.

   ```yaml
   apiVersion: eventing.knative.dev/v1alpha1
   kind: Channel
   metadata:
     name: my-kafka-channel
   spec:
     provisioner:
       apiVersion: eventing.knative.dev/v1alpha1
       kind: ClusterChannelProvisioner
       name: kafka
   ```

## Components

The major components are:

- ClusterChannelProvisioner Controller
- Channel Controller
- Channel Controller Config Map.
- Channel Dispatcher
- Channel Dispatcher Config Map.

The ClusterChannelProvisioner Controller and the Channel Controller are
colocated in one Pod:

```shell
kubectl get deployment -n knative-eventing kafka-channel-controller
```

The Channel Controller Config Map is used to configure the `bootstrap_servers`
of your Apache Kafka installation:

```shell
kubectl get configmap -n knative-eventing kafka-channel-controller-config
```

The Channel Dispatcher receives and distributes all events:

```shell
kubectl get statefulset -n knative-eventing kafka-channel-dispatcher
```

The Channel Dispatcher Config Map is used to send information about Channels and
Subscriptions from the Channel Controller to the Channel Dispatcher:

```shell
kubectl get configmap -n knative-eventing kafka-channel-dispatcher
```
