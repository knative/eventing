# Knative Eventing Performance Tests

## Configuring your cluster to run a benchmark

1. Create a namespace `perf-eventing` if it does not exist. To use a different
   namespace, please replace all the namespaces in all bash commands and yaml
   configuration files with your choice.

1. Install Knative eventing and components used in the performance test, such as
   MT broker, by following the steps in
   https://github.com/knative/eventing/blob/main/DEVELOPMENT.md.

1. Create a ConfigMap called `config-mako` in your chosen namespace containing
   the Mako config file.

```
kubectl create configmap -n perf-eventing config-mako --from-file=test/performance/benchmarks/<benchmark>/dev.config
```

1. Optionally edit the ConfigMap to set additional keys.

   ```
   kubectl edit configmap -n perf-eventing config-mako
   ```

[`NewConfigFromMap`](https://github.com/knative/pkg/blob/main/test/mako/config.go#L41)
determines the valid keys in this ConfigMap. Current keys are:

- `environment`: Select a Mako config file in the ConfigMap. E.g.
  `environment: dev` corresponds to `dev.config`.
- `additionalTags`: Comma-separated list of tags to apply to the Mako run.

### Run benchmarks continuously in CI using Mako

To run a benchmark continuously, and make the result available on
[Mako](https://mako.dev/project?name=Knative):

1.  Use `ko` to apply yaml files in the benchmark directory.

    ```
    ko apply -f test/performance/benchmarks/broker-imc/continuous
    ```

### Run without Mako

To run a benchmark once, and use the result from `mako-stub` for plotting:

1. Install the eventing resources for attacking:

   ```
   ko apply -f test/performance/benchmarks/broker-imc/100-broker-perf-setup.yaml
   ```

1. Start the benchmarking job:

   ```
   ko apply -f test/performance/benchmarks/broker-imc/200-broker-perf.yaml
   ```

1. Wait until all the pods in namespace `perf-eventing` are completed.

1. Retrieve results from mako-stub using the script in
   [knative/pkg](https://github.com/knative/pkg/blob/main/test/mako/stub-sidecar/read_results.sh)
   where `pod_name` is the name of the aggregator pod:

   ```
   bash "$GOPATH/src/knative.dev/eventing/vendor/knative.dev/pkg/test/mako/stub-sidecar/read_results.sh" "$pod_name" perf-eventing ${mako_port:-10001} ${timeout:-120} ${retries:-100} ${retries_interval:-10} "$output_file"
   ```

   This will download a CSV with all raw results. Alternatively you can remove
   the port argument `-p` in `mako-stub` container to dump the output to
   container log directly.

## Available benchmarks

- `direct`: Source -> Sink (baseline test)
- `broker-imc`: Source -> Broker with IMC -> Sink
- `channel-imc`: Source -> IMC -> Sink

## Plotting results from mako-stub

In order to plot results from the mako-stub, you need to have installed
`gnuplot`.

Three plot scripts are available:

- Only send/receive latencies
- Only send/receive throughput
- Combined send/receive throughput

To use them, you need to pass as first parameter the csv. If you want to use the
combined plot script, you need to specify also latency upper bound, thpt lower
and upper bound to show. For example:

```
gnuplot -c test/performance/latency-and-thpt-plot.plg data.csv 0.005 480 520
```

## Profiling

Most eventing binaries under `cmd` package are bootstrapped by either
`sharedmain.Main` in `knative.dev/pkg/injection/sharedmain` or `adapter.Main` in
`knative.dev/eventing/pkg/adapter`. These `Main` helper functions uses the
[profiling](https://github.com/knative/pkg/blob/main/profiling/server.go)
package to enable golang profiling by reading the `profiling.enable` flag in the
`config-observability` configmap.

To enable profiling,

1. Add or modify `profiling.enable: "true"` in
   `config/config-observability.yaml`'s `data` field and apply the change. Or
   use `kubectl edit configmap -n knative-eventing config-observability`.
2. Port forward into the pod which you want to profile, e.g.,
   `kubectl port-forward <imc-dispatcher-pod> 8008:8008`
3. Point your browser to `http://localhost:8008/debug/pprof/` and view pprof
   data.

After you are done, you can disable profiling by setting
`profiling.enable: "false"`.
