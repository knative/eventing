# Latency test image

This image is designed to benchmark Knative Eventing channel/brokers.

The image does both the sender and receiver role, allowing the clock to be synchronized to correctly calculate latencies.

Latencies are calculated and published to the mako sidecar container.

The image is designed to allocate as much memory as possible before the benchmark starts. We suggest to disable Go GC to avoid useless GC pauses.

## Usage

Example of how to use this image with mako stub sidecar:

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
        - containerPort: 8080
      args:
        - "--sink=http://in-memory-test-broker-broker.perf-eventing.svc.cluster.local"
        - "--pace=100:10,200:20,400:60"
        - "--warmup=10"
        - "--verbose"
      volumeMounts:
        - name: config-mako
          mountPath: /etc/config-mako
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

`pace` is a comma separated array of pace configurations in format `rps[:duration=10s]`.

For example the configuration `100,200:20,400:60` means:

1. 100 rps for 10 seconds
2. 200 rps for 20 seconds
3. 400 rps for 60 seconds

### Warmup phase

You can configure a warmup phase to warm the hot path of channel implementations. This is especially required while working with JVM or similar environments. During the warmup phase, no latencies are calculated.

To configure the duration of warmup phase, use flag `warmup` specifying the number of seconds.

If you don't want a warmup phase, use `--warmup=0`.

### Workers

You can specify the number of vegeta workers that perform requests with flag `workers`.
