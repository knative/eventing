# Samples

This directory contains sample services which demonstrate `Feed`
functionality.

## Prerequisites

1. [Setup your development environment](../DEVELOPMENT.md#getting-started)
2. [Start Eventing](../DEVELOPMENT.md#starting-eventing-controller)

## Tools

- [Kail](https://github.com/boz/kail) - Kubernetes tail. Streams logs from all
  containers of all matched pods. Match pods by service, replicaset,
  deployment, and others. Adjusts to a changing cluster - pods are added and
  removed from logging as they fall in or out of the selection.


## Samples

See [the docs repo](https://github.com/knative/docs/tree/master/eventing/samples) for the best-maintained samples:

* [Handling Kubernetes events](https://github.com/knative/docs/tree/master/eventing/samples/k8s-events) -
  A simple handler for processing k8s events from the local cluster.
* [Binding running services to an IoT core](https://github.com/knative/docs/tree/master/eventing/samples/event-flow) -
  A sample using Google PubSub to read events from Google's IoT core.
* [Github Pull Request Handler](https://github.com/knative/docs/tree/master/eventing/samples/github-events) -
  A simple handler for Github Pull Requests
