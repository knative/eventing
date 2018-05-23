# gcp_pubsub

An example of a Receive Adapter that creates a Pull based subscriber to GCP PubSub.

**Proof of Concept only** we need to containerize this, for now this lives "in-proc" with
the controller, but wanted to use the same concept as a github EventSource and then in a
follow on move to a container based out of proc model.

This lives in the sample as conceptually this is not part of the core controller model.
This just gets linked into the controller so there's nothing to deploy here yet.

# Installing

Just install the CRDs like this:

```shell
$ kubectl create -f sample/gcp_pubsub/eventsource.yaml
$ kubectl create -f sample/gcp_pubsub/eventtype.yaml
```
