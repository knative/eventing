# Upgrade script required to upgrade to v0.14.0 of Eventing

This directory contains a job that upgrades (patches) all the Brokers to have an
annotation that specifies which BrokerClass should be reconciling it. Prior to
v0.13.0 there was only the ChannelBasedBroker and it was not necessary. However
in v0.14.0 we introduced BrokerClass and in order for existing Brokers to
continue to be reconciled, they need to be patched to include the BrokerClass
annotation like so.

```
  annotations:
    eventing.knative.dev/broker.class: ChannelBasedBroker

```

To run the upgrade script:

```shell
kubectl apply -f https://github.com/knative/eventing/releases/download/v0.14.0/upgrade-to-v0.14.0.yaml
```

It will create a job called v0.14.0-upgrade in the knative-eventing namespace.
If you installed to different namespace, you need to modify the upgrade.yaml
appropriately. Also the job by default runs as `eventing-controller` service
account, you can also modify that but the service account will need to have
permissions to list `Namespace`s, list and patch `Broker`s.
