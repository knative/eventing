# Stub - Knative Bus

Deployment steps:
1. Setup [Knative Eventing](../../../DEVELOPMENT.md)
1. For cluster wide deployment, change the kind in `config/buses/stub/stub-bus.yaml` from `Bus` to `ClusterBus`.
1. Apply the 'stub' Bus `ko apply -f config/buses/stub/`
1. Create Channels that reference the 'stub' Bus
1. (Optional) Install [Kail](https://github.com/boz/kail) - Kubernetes tail

The bus is only a dispatcher.

The dispatcher receives events via a Channel's Service from inside the cluster and forwarded via HTTP to the subscribers.

Note: The stub bus does not guarantee delivery, errors will not be reattempted.

To view logs: `kail -d stub-bus -c dispatcher`
