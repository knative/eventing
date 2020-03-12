# Knative Eventing Scaling

## Introduction

Kantive Eventing scaling is designed to be compatible with
[Knative Serving Scaling](https://github.com/knative/serving/tree/master/docs/scaling).
That should make it easy to apply consistent scaling configurations across whole
Knative.

## Scale Bounds

There are cases when Operators need to set lower and upper bounds on the number
of pods serving their apps (e.g. avoiding cold-start, control compute costs,
etc).

The following annotations can be used in Knative Eventing sources, channles and
brokers to do exactly that:

```yaml
# +optional
# When not specified, minimum scale is one, use to enable scale to zero
autoscaling.knative.dev/minScale: "0"
# +optional
# When not specified, the upper scale bound is one
autoscaling.knative.dev/maxScale: "10"
```

**NOTE**: the scaling defaults are different in Knative Eventing than in
Scaling. If annotations are not provided default for Kantive Eventing resources
is to have minimum and maximum scale set to one i.e. not scaling.

The defaults can be changed by creating default configuration in cluster scope
and also to create default per-namespace. (TODO describe mechanism)

Default
[Knative Serving autoscaling annotations](https://knative.dev/docs/serving/configuring-autoscaling/)
SHOULD be used when underlying implementation is using Knative Serving (that may
be the best option for push-based event sources if they are implemented using
Knative Serving).

## Extensibility

Similar to
[Knative Serving](https://knative.dev/docs/serving/configuring-autoscaling/#implementing-your-own-pod-autoscaler)
class annotation `autoscaling.knative.dev/class` can be used to specify
implementation of eventing scaling.

Event sources that are using [KEDA](https://keda.sh/) for scaling we have
defined annotation extension:
[KEDA Knative Eventing Scaling description](./KEDA.md).

### Push-based Event Sources

List of Knative Eventing push-based event sources that may support scaling:

- [Github Event Source](https://github.com/knative/eventing-contrib/tree/master/github)

### Push-based Event Sources

List of Knative Eventing push-based event sources that support scaling:

- [Google PubSub Event Source](https://github.com/google/knative-gcp/pull/551)
- [Kafka Event Source](https://github.com/knative/eventing-contrib/pull/886)

### Event Channels

List of Knative Eventing push-based event sources

- None

### Event Brokers

List of Knative Eventing push-based event sources

- None
