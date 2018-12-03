# NATS Streaming Channels


### Deployment steps:

1. Setup [Knative Eventing](../../../DEVELOPMENT.md).
1. ```sbtshell
   kubectl create namespace natss
   kubectl label namespace natss istio-injection=enabled
   ``` 
1. Apply the 'natss' ClusterChannelProvisioner, Controller, and Dispatcher.
     ```shell
     ko apply -f config/provisioners/natss/
     ````
1. Create Channels that reference the 'natss'.

    ```yaml
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Channel
    metadata:
      name: foo
    spec:
      provisioner:
        apiVersion: eventing.knative.dev/v1alpha1
        kind: ClusterChannelProvisioner
        name: natss
    ```

### Components

The major components are:
* NATS Streaming
* ClusterChannelProvisioner Controller
* Channel Controller
* Channel Dispatcher

NATS Streaming is deployed as a StatefulSet. 
For tuning NATS Streaming, see:
https://github.com/nats-io/nats-streaming-server#configuring

The ClusterChannelProvisioner Controller and the Channel Controller are colocated in one Pod.
```shell
kubectl get deployment -n knative-eventing natss-controller
```

The Channel Dispatcher receives and distributes all events. There is a single Dispatcher for all
natss Channels.
```shell
kubectl get deployment -n knative-eventing natss-dispatcher
```

