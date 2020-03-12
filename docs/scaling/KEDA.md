# Knative Eventing KEDA Scaling

## Introduction

This document defines Knative Eventing extension to autoscaling annotations to
support KEDA. [Kubernetes Event-driven Autoscaling (KEDA)](https://keda.sh/)
provides scaling of any container in Kubernetes based on the number of events
needing to be processed. We use it in Knative Eventing to scale pull-based event
sources.

## Using KEDA scaling in Knative Eventing

Supported event sources (and other components in future) may indicate that they
support Knative Eventing scaling with KEDA.

To enable using KEDA scalign add Knative Eventing scaling class:

```yaml
    autoscaling.knative.dev/class: keda.autoscaling.knative.dev
```

and additional KEDA configuration parameters may be enabled with
`keda.autoscaling.knative.dev/NAME` annotations.

For example:

```yaml
keda.autoscaling.knative.dev/pollingInterval: "2"
keda.autoscaling.knative.dev/cooldownPeriod: "15"
```

List of Knative Eventing sources that support KEDA scaling:
* [Google PubSub](https://github.com/google/knative-gcp/pull/551)
* [Kafka Source](https://github.com/knative/eventing-contrib/pull/886)
