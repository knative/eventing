# GCP Cloud Pub/Sub - Knative Bus

Deployment steps:
1. Setup [Knative Eventing](../../../DEVELOPMENT.md)
1. [Create a service account](https://console.cloud.google.com/iam-admin/serviceaccounts/project) with the 'Pub/Sub Editor' role, and download a new JSON private key.
1. Create a secret for the downloaded key `kubectl create secret generic gcppubsub-bus-key --from-file=key.json=PATH-TO-KEY-FILE.json`
1. Configure the bus, replacing `$PROJECT_ID` with your GCP Project ID, `kubectl create configmap gcppubsub-bus-config --from-literal=GOOGLE_CLOUD_PROJECT=$PROJECT_ID`
1. For cluster wide deployment, change the kind in `config/buses/gcppubsub/gcppubsub-bus.yaml` from `Bus` to `ClusterBus`.
1. Apply the 'gcppubsub' Bus `ko apply -f config/buses/gcppubsub/`
1. Create Channels that reference the 'gcppubsub' Bus
1. (Optional) Install [Kail](https://github.com/boz/kail) - Kubernetes tail

The bus has an independent provisioner and dispatcher.

The provisioner will create GCP Pub/Sub Topics and Subscriptions for each Knative Channel and Subscription (respectively) targeting the Bus. Clients should avoid interacting with topics and subscriptions provisioned by the Bus.

The dispatcher receives events via a Channel's Service from inside the cluster and sends them to the Pub/Sub Topic. Events on the Pub/Sub topic for an active subscription are forwarded via HTTP to the subscribers. HTTP responses with a 2xx status code are ack'ed while all other status codes will nack the event, delivery will be reattempted up to the limits defined by Cloud Pub/Sub.

Note: Cloud Pub/Sub does not guarantee exactly once delivery, subscribers must guard against multiple deliveries of the same event.

To view logs:
- for the dispatcher `kail -d gcppubsub-bus-dispatcher -c dispatcher`
- for the provisioner `kail -d gcppubsub-bus-provisioner -c provisioner`
