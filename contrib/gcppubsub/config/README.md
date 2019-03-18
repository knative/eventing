# GCP PubSub Channels

GCP PubSub channels are production-quality Channels that are backed by
[GCP PubSub](https://cloud.google.com/pubsub/).

They offer:

- Persistence
  - If the Channel's Pod goes down, all events already ACKed by the Channel will
    persist and be retransmitted when the Pod restarts.
- Redelivery attempts
  - If downstream rejects an event, that request is attempted again.

They do not offer:

- Ordering guarantees
  - Events seen downstream may not occur in the same order they were inserted
    into the Channel.

### Deployment Steps

#### Prerequisites

1. Create a (or use existing)
   [Google Cloud Project](https://cloud.google.com/resource-manager/docs/creating-managing-projects).

   Then set an env variable that we'll use in other commands below.

   ```shell
   export PROJECT_ID='YOUR_PROJECT_ID_HERE'
   ```

1. Enable the 'Cloud Pub/Sub API' on that project.

   ```shell
   gcloud services enable pubsub.googleapis.com
   ```

1. Create a GCP
   [Service Account](https://console.cloud.google.com/iam-admin/serviceaccounts/project).

   1. Determine the Service Account to use, or create a new one.

   If using existing service account, just set an env variable.

   ```shell
   export PUBSUB_SERVICE_ACCOUNT='YOUR_SERVICE_ACCOUNT_NAME'
   ```

   If creating a new service account:

   ```shell
   export PUBSUB_SERVICE_ACCOUNT='YOUR_NEW_SERVICE_ACCOUNT_NAME'
   gcloud iam service-accounts create $PUBSUB_SERVICE_ACCOUNT
   ```

   1. Give that Service Account the 'Pub/Sub Editor' role on your GCP project.

      ```shell
      gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member=serviceAccount:$PUBSUB_SERVICE_ACCOUNT@$PROJECT_ID.iam.gserviceaccount.com \
        --role roles/pubsub.editor
      ```

   1. Download a new JSON private key for that Service Account. **Be sure not to
      check this key into source control!**

      ```shell
      gcloud iam service-accounts keys create knative-gcppubsub-channel.json \
        --iam-account=$PUBSUB_SERVICE_ACCOUNT@$PROJECT_ID.iam.gserviceaccount.com
      ```

   1. Create a secret for the downloaded key:

      ```shell
      kubectl -n knative-eventing create secret generic gcppubsub-channel-key --from-file=key.json=knative-gcppubsub-channel.json
      ```

1. Setup [Knative Eventing](../../../DEVELOPMENT.md).

#### Deployment

1. Apply `gcppubsub.yaml`.

   ```shell
   sed "s/REPLACE_WITH_GCP_PROJECT/$PROJECT_ID/" contrib/gcppubsub/config/gcppubsub.yaml | ko apply -f -
   ```

1. Create Channels that reference the `gcp-pubsub` Channel. Create a file like
   so (save it for example in /tmp/channel.yaml):

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

1. Then create the actual Channel resource:

   ```shell
   kubectl create -f /tmp/channel.yaml
   ```

1. Then inspect the created resource and make sure it is reported as Ready

   ```shell
   kubectl get channels foo -oyaml
   ```

   And you should see something like this:

   ```shell
   vaikas@penguin:~/projects/go/src/github.com/knative/eventing$ kubectl get channels foo -oyaml
   apiVersion: eventing.knative.dev/v1alpha1
   kind: Channel
   metadata:
     creationTimestamp: 2019-01-29T22:58:52Z
     finalizers:
     - gcp-pubsub-channel-controller
     - gcp-pubsub-channel-dispatcher
     generation: 1
     name: foo
     namespace: default
     resourceVersion: "4681440"
     selfLink: /apis/eventing.knative.dev/v1alpha1/namespaces/default/channels/foo
     uid: 7261543f-2419-11e9-bcc1-42010a8a0004
   spec:
     generation: 1
     provisioner:
       apiVersion: eventing.knative.dev/v1alpha1
       kind: ClusterChannelProvisioner
       name: gcp-pubsub
   status:
     address:
       hostname: foo-channel-cshj8.default.svc.cluster.local
     conditions:
     - lastTransitionTime: 2019-01-29T22:58:54Z
       severity: Error
       status: "True"
       type: Addressable
     - lastTransitionTime: 2019-01-29T22:58:54Z
       severity: Error
       status: "True"
       type: Provisioned
     - lastTransitionTime: 2019-01-29T22:58:54Z
       severity: Error
       status: "True"
       type: Ready
     internal:
       gcpProject: quantum-reducer-434
       secret:
         apiVersion: v1
         kind: Secret
         name: gcppubsub-channel-key
         namespace: knative-eventing
       secretKey: key.json
       topic: knative-eventing-channel_foo_7261543f-2419-11e9-bcc1-42010a8a0004
   ```

### Components

The major components are:

- [Channel Controller](../../../contrib/gcppubsub/pkg/controller)
  - [ClusterChannelProvisioner Controller](../../../contrib/gcppubsub/pkg/clusterchannelprovisioner)
  - [Channel Controller](../../../contrib/gcppubsub/pkg/channel)
- [Channel Dispatcher](../../../contrib/gcppubsub/pkg/dispatcher/cmd)
  - [Dispatcher](../../../contrib/gcppubsub/pkg/dispatcher/dispatcher)
  - [Receiver](../../../contrib/gcppubsub/pkg/dispatcher/receiver)

The `Channel Controller` controls all the Kubernetes resources and creates
`Topic`s and `Subscription`s in GCP PubSub. It runs in the Deployment:

```shell
kubectl get deployment -n knative-eventing gcp-pubsub-channel-controller
```

The `Channel Dispatcher` handles all the data plane portions of the `Channel`.
It receives events from the cluster and writes them to GCP PubSub `Topic`s. It
also polls `Subscriptions`s and sends events back into the cluster. It runs in
the Deployment:

```shell
kubectl get deployment -n knative-eventing gcp-pubsub-channel-dispatcher
```
