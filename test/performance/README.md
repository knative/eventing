# Knative Eventing Performance Tests

## Getting Started

1. Create a namespace or use an existing namespace.

Create a ConfigMap called `config-mako` in
your chosen namespace.

```
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-mako
data:
  environment: dev
EOF
```

## Running a benchmark

1. Use `ko` to apply yaml files in the benchmark directory.

```
ko apply -f test/performance/broker-latency`
```