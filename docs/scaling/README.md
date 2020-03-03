# Knative Eventing Scaling

## Introduction

Kantive Eventing scaling is designed to be compatible with [Knative Serving Scaling](https://github.com/knative/serving/tree/master/docs/scaling). That should make it easy to apply consistent scaling configurations across whole Knative.

## Scale Bounds

There are cases when Operators need to set lower and upper bounds on the number
of pods serving their apps (e.g. avoiding cold-start, control compute costs, etc).

The following annotations can be used in Knative Eventing sources, channles and brokers to do exactly that:

```yaml
# +optional
# When not specified, minimum scale is one, use to enable scale to zero
autoscaling.knative.dev/minScale: "0"
# +optional
# When not specified, the upper scale bound is one
autoscaling.knative.dev/maxScale: "10"
```

**NOTE**: the scaling defaults are different in Knative Eventing than in Scaling. If annotations are not provided default for Kantive Eventing resources is to have minimum and maximum scale set to one i.e. not scaling.

## Extensibility

Similar to [Knative Serving](https://knative.dev/docs/serving/configuring-autoscaling/#implementing-your-own-pod-autoscaler) class annotation `autoscaling.knative.dev/class` can be used to specify implementation of eventing scaling.

The default autoscaler implemention supported in Knative Eventing requires [KEDA](https://keda.sh/). For details read [KEDA Knative Eventing Scaling description](./KEDA.md).
