# GCP PubSub Channels

GCP PubSub channels are production-quality Channels that are backed by
[GCP PubSub](https://cloud.google.com/pubsub/).

They offer:
* Persistence
    - If the Channel's Pod goes down, all events already ACKed by the Channel will persist and be
      retransmitted when the Pod restarts.
* Redelivery attempts
    - If downstream rejects an event, that request is attempted again.
 
They do not offer:
* Ordering guarantees
    - Events seen downstream may not occur in the same order they were inserted into the Channel.
    
### Deployment Steps

#### Prerequisites

1. Create a [Google Cloud Project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).
1. Enable the 'Cloud Pub/Sub API' on that project.

    ```shell
    gcloud services enable pubsub.googleapis.com
    ```

1. Create a GCP [Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project).
    1. Determine the Service Account to use, or create a new one.
    1. Give that Service Account the 'Pub/Sub Editor' role on your GCP project.
    1. Download a new JSON private key for that Service Account.
    1. Create a secret for the downloaded key:
        
        ```shell
        kubectl -n knative-sources create secret generic gcppubsub-channel-key --from-file=key.json=PATH_TO_KEY_FILE.json
        ```

1. Setup [Knative Eventing](../../../DEVELOPMENT.md).

#### Deployment

1. Set the shell variable with the correct value:

    ```shell
    export GCP_PROJECT=REPLACE_ME
    ```

1. Apply `gcppubsub.yaml`.

    ```shell
    sed "s/REPLACE_WITH_GCP_PROJECT/$GCP_PROJECT/" config/provisioners/gcppubsub/gcppubsub.yaml | ko apply -f -
    ```

1. Create Channels that reference the `gcp-pubsub` Channel.

    ```yaml
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Channel
    metadata:
      name: foo
    spec:
      provisioner:
        apiVersion: eventing.knative.dev/v1alpha1
        kind: ClusterChannelProvisioner
        name: gcp-pubsub
    ```

### Components

The major components are:
* [Channel Controller](../../../pkg/provisioners/gcppubsub/controller)
    - [ClusterChannelProvisioner Controller](../../../pkg/provisioners/gcppubsub/clusterchannelprovisioner)
    - [Channel Controller](../../../pkg/provisioners/gcppubsub/channel)
* [Channel Dispatcher](../../../pkg/provisioners/gcppubsub/dispatcher/cmd)
    - [Dispatcher](../../../pkg/provisioners/gcppubsub/dispatcher/dispatcher)
    - [Receiver](../../../pkg/provisioners/gcppubsub/dispatcher/receiver)
    
The `Channel Controller` controls all the Kubernetes resources and creates `Topic`s and
`Subscription`s in GCP PubSub. It runs in the Deployment:

```shell
kubectl get deployment -n knative-eventing gcp-pubsub-channel-controller
```

The `Channel Dispatcher` handles all the data plane portions of the `Channel`. It receives events
from the cluster and writes them to GCP PubSub `Topic`s. It also polls `Subscriptions`s and sends
events back into the cluster. It runs in the Deployment:

```shell
kubectl get deployment -n knative-eventing gcp-pubsub-channel-dispatcher
```
