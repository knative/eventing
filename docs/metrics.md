# Metrics

This is a list of metrics exported by Knative Eventing components.

## Broker

These are exported by `broker-ingress` pods.

| Name                   | Type      | Description                | Tags               |
| ---------------------- | --------- | -------------------------- | ------------------ |
| `broker_events_total`  | count     | Number of events received. | `result`, `broker` |
| `broker_dispatch_time` | histogram | Time to dispatch an event. | `result`, `broker` |

## Trigger

These are exported by `broker-filter` pods.

| Name                    | Type      | Description                | Tags                                           |
| ----------------------- | --------- | -------------------------- | ---------------------------------------------- |
| `trigger_events_total`  | count     | Number of events received. | `result`, `broker`, `trigger`                  |
| `trigger_dispatch_time` | histogram | Time to dispatch an event. | `result`, `broker`, `trigger`                  |
| `trigger_filter_time`   | histogram | Time to filter an event.   | `result`, `broker`, `trigger`, `filter_result` |
| `broker_to_function_delivery_time` | histogram | Time from ingress of an event until it is dispatched | `result`, `broker`, `trigger` |

## Access metrics

Accessing metrics requires Prometheus and Grafana installed. Follow the
[instructions to install Prometheus and Grafana](https://github.com/knative/docs/blob/master/docs/serving/installing-logging-metrics-traces.md)
in namespace `knative-monitoring`.

### Prometheus

Follow the
[instructions to open Prometheus UI](https://github.com/knative/docs/blob/master/docs/serving/accessing-metrics.md#prometheus),
then you will access the metrics at
[http://localhost:9090](http://localhost:9090).

### Grafana

You can add the eventing dashboard to Grafana by running below command line
under the root folder of `eventing` source code:

```
kubectl patch configmap grafana-dashboard-definition-knative --patch "$(cat config/monitoring/metrics/grafana/100-grafana-dash-eventing.yaml)" -n knative-monitoring
```

Follow the
[instructions to open Grafana dashboard](https://github.com/knative/docs/blob/master/docs/serving/accessing-metrics.md#grafana),
then you will access the metrics at
[http://localhost:3000](http://localhost:3000).
