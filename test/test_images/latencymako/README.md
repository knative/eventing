# Latency test image

This image is designed to benchmark Knative Eventing channel/brokers.

The image does both the sender and receiver role, allowing the clock to be
synchronized to correctly calculate latencies (only valid with a single
sender-receiver).

Latencies are calculated and published to the Mako sidecar container by a
separate aggregator.

The image is designed to allocate as much memory as possible before the
benchmark starts. We suggest to disable Go GC to avoid useless GC pauses.

## Usage

Example of how to use this image with the Mako stub sidecar:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: latency-test
  namespace: perf-eventing
  labels:
    role: latency-test-consumer
spec:
  serviceAccountName: default
  restartPolicy: Never
  containers:
    - name: latency-test
      image: knative.dev/eventing/test/test_images/latencymako
      resources:
        requests:
          cpu: 1000m
          memory: 2Gi
      ports:
        - name: cloudevents
          containerPort: 8080
      args:
        - "--role=sender-receiver"
        - "--sink=http://in-memory-test-broker-broker.perf-eventing.svc.cluster.local"
        - "--aggregator=localhost:10000"
        - "--pace=100:10,200:20,400:60"
        - "--warmup=10"
        - "--verbose"
      env:
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
      volumeMounts:
        - name: config-mako
          mountPath: /etc/config-mako
      terminationMessagePolicy: FallbackToLogsOnError
    - name: aggregator
      image: knative.dev/eventing/test/test_images/latencymako
      ports:
        - name: grpc
          containerPort: 10000
      args:
        - "--role=aggregator"
        - "--verbose"
      terminationMessagePolicy: FallbackToLogsOnError
    - name: mako-stub
      image: knative.dev/pkg/test/mako/stub-sidecar
      terminationMessagePolicy: FallbackToLogsOnError
  volumes:
    - name: config-mako
      configMap:
        name: config-mako
```

There are two required flags: the `sink` and `pace`.

### Pace configuration

`pace` is a comma separated array of pace configurations in format
`rps[:duration=10s]`.

For example the configuration `100,200:20,400:60` means:

1. 100 rps for 10 seconds
2. 200 rps for 20 seconds
3. 400 rps for 60 seconds

### Warmup phase

You can configure a warmup phase to warm the hot path of channel
implementations. This is especially required while working with JVM or similar
environments. During the warmup phase, no latencies are calculated.

To configure the duration of warmup phase, use flag `warmup` specifying the
number of seconds.

If you don't want a warmup phase, use `--warmup=0`.

### Workers

You can specify the number of vegeta workers that perform requests with flag
`workers`.
