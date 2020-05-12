# Upgrade script (optional) to upgrade to v0.15.0 of Eventing

This directory contains a job that updates all of v1alpha1 based Brokers that
are using Spec.ChannelTemplate to specify which channel they use. You are not
able to create new ones specifying the ChannelTemplate, and you can either
manually update them yourself, or run the tool that will do the following for
any Broker that is using Spec.ChannelTemplate:

1. Create a ConfigMap in the same namespace as the Broker named:
   `broker-auto-gen-config-<brokername>` with the content from Spec.ChannelTemplate.
1. Set Broker Spec.Config to point to this ConfigMap
1. Set Broker Spec.ChannelTemplate to nil

To run the upgrade script:

```shell
kubectl apply -f https://github.com/knative/eventing/releases/download/v0.15.0/upgrade-to-v0.15.0.yaml
```

It will create a job called v0.15.0-upgrade in the knative-eventing namespace.
If you installed to different namespace, you need to modify the upgrade.yaml
appropriately. Also the job by default runs as `eventing-controller` service
account, you can also modify that but the service account will need to have
permissions to list `Namespace`s, list and patch `Broker`s, and create
`ConfigMap`s.
