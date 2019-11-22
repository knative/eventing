# Knative Eventing Performance Tests

## Getting Started

1. Install Knative eventing by following the steps in
https://github.com/knative/eventing/blob/2c6bf0526634804b7ebeee686445901440cc8edd/test/performance/performance-tests.sh#L31

## Running a benchmark

To run a benchmark continuously, and make the result available on [Mako](https://mako.dev/project?name=Knative):

1. Create a ConfigMap called `config-mako`, as described in
https://github.com/knative/eventing/blob/master/test/performance/config/config-mako.yaml.

    > Before `kubectl apply` the ConfigMap, the value of `dev.config` needs to by replaced with
    > dev.config of the benchmark you want to run.

1.  Use `ko` to apply yaml files in the benchmark directory.

    ```
    ko apply -f test/performance/benchmarks/broker-imc/continuous
    ```

To run a benchmark once, and use the result from `mako-stub` for plotting:

1. Install the eventing resources for attacking:

    ```
    ko apply -f test/performance/benchmarks/broker-imc/100-broker-perf-setup.yaml
    ```

1. Start the benchmarking job:

    ```
    ko apply -f test/performance/benchmarks/broker-imc/200-broker-perf.yaml
    ```

## Available benchmarks

- `direct`: Source -> Sink (baseline test)
- `broker-imc`: Source -> Broker IMC -> Sink
- `channel-imc`: Source -> Channel IMC -> Sink

## Plotting results from mako-stub

In order to plot results from the mako-stub, you need to have installed
`gnuplot`.

First you need to collect results from logs:

```
kubectl logs -n perf-eventing direct-perf-aggregator mako-stub -f > data.csv
```

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
