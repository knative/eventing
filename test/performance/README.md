# Knative Eventing Performance Tests

## Configuring your cluster to run a benchmark

1. Create a namespace or use an existing namespace. Each namespace can be
   configured with a single benchmark.

1. Install Knative eventing by following the steps in
   https://github.com/knative/eventing/blob/2c6bf0526634804b7ebeee686445901440cc8edd/test/performance/performance-tests.sh#L31

1. Create a ConfigMap called `config-mako` in your chosen namespace containing
   the Mako config file.

```
kubectl create configmap -n <namespace> config-mako --from-file=test/performance/benchmarks/<benchmark>/dev.config
```

1. Optionally edit the ConfigMap to set additional keys.

   ```
   kubectl edit configmap -n <namespace> config-mako
   ```

[`NewConfigFromMap`](https://github.com/knative/pkg/blob/master/test/mako/config.go#L41)
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

1. Retrieve results from mako-stub using the script in
   [knative/pkg](https://github.com/knative/pkg/blob/master/test/mako/stub-sidecar/read_results.sh):

   ```
   bash "$GOPATH/src/knative.dev/pkg/test/mako/stub-sidecar/read_results.sh" "$pod_name" perf-eventing ${mako_port:-10001} ${timeout:-120} ${retries:-100} ${retries_interval:-10} "$output_file"
   ```

   This will download a CSV with all raw results.

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
