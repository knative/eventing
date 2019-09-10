# Knative Eventing Performance Tests

## Getting Started

1.  Create a namespace or use an existing namespace.

Create a ConfigMap called `config-mako` in your chosen namespace.

```
cat <<EOF | kubectl apply -n <namespace> -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-mako
data:
  environment: dev
EOF
```

[`NewConfigFromMap`](https://github.com/knative/pkg/blob/master/test/mako/config.go#L41)
determines the valid keys in this ConfigMap. Current keys are:

*   `environment`: Selects a Mako config file in kodata. E.g. `environment: dev`
    corresponds to `kodata/dev.config`.
*   `additionalTags`: Comma-separated list of tags to apply to the Mako run.

## Running a benchmark

1.  Use `ko` to apply yaml files in the benchmark directory.

```
ko apply -f test/performance/broker-latency
```
