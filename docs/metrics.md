# Metrics

This is a list of data-plane metrics exported by Knative Eventing components.

## Broker

These are exported by `broker-ingress` pods.

| Name                   | Type      | Description                | Tags               |
| ---------------------- | --------- | -------------------------- | ------------------ |
| `event_count`  | count     | Number of events received by a Broker. | `namespace_name`, `broker_name`, `event_source`, `event_type`, `response_code`, `response_code_class` |
| `event_dispatch_latencies` | histogram | The time spent dispatching an event to a Channel. | `namespace_name`, `broker_name`, `event_source`, `event_type`, `response_code`, `response_code_class` |

## Trigger

These are exported by `broker-filter` pods.

| Name                               | Type      | Description                                          | Tags                                           |
| ---------------------------------- | --------- | ---------------------------------------------------- | ---------------------------------------------- |
| `event_count`             | count     | Number of events received by a Trigger                           | `namespace_name`, `trigger_name`, `broker_name`, `filter_source`, `filter_type`, `response_code`, `response_code_class` |
| `event_dispatch_latencies`            | histogram | The time spent dispatching an event to a Trigger subscriber                           | `namespace_name`, `trigger_name`, `broker_name`, `filter_source`, `filter_type`, `response_code`, `response_code_class` |
| `event_processing_latencies` | histogram | The time spent processing an event before it is dispatched to a Trigger subscriber | `namespace_name`, `trigger_name`, `broker_name`, `filter_source`, `filter_type` |

## Sources

These are exported by core sources.

### ApiServerSource

| Name                               | Type      | Description                                          | Tags                                           |
| ---------------------------------- | --------- | ---------------------------------------------------- | ---------------------------------------------- |
| `event_count`             | count     | Number of events sent                           | `namespace_name`, `source_name`, `source_resource_group`, `event_source`, `event_type`, `response_code`, `response_code_class` |

### CronJobSource

| Name                               | Type      | Description                                          | Tags                                           |
| ---------------------------------- | --------- | ---------------------------------------------------- | ---------------------------------------------- |
| `event_count`             | count     | Number of events sent                           | `namespace_name`, `source_name`, `source_resource_group`, `event_source`, `event_type`, `response_code`, `response_code_class` |


# Access metrics

## Prometheus Collection

Accessing metrics requires Prometheus and Grafana installed. Follow the
[instructions to install Prometheus and Grafana](https://github.com/knative/docs/blob/master/docs/serving/installing-logging-metrics-traces.md)
in namespace `knative-monitoring`.

> _All commands assume root of repo._

1. Enable Knatives install of Prometheus to scrape Knative Eventing, run the
   following:

   ```shell
   kubectl get configmap -n  knative-monitoring prometheus-scrape-config -oyaml > tmp.prometheus-scrape-config.yaml
   sed -e 's/^/    /' config/monitoring/metrics/prometheus/100-prometheus-scrape-kn-eventing.yaml > tmp.prometheus-scrape-kn-eventing.yaml
   sed -e '/    scrape_configs:/r tmp.prometheus-scrape-kn-eventing.yaml' tmp.prometheus-scrape-config.yaml \
     | kubectl apply -f -
   ```

2. To verify, run the following to show the diff between what the resource was
   original and is now (_note_: k8s will update `metadata.annotations`):

   ```shell
   kubectl get configmap -n  knative-monitoring prometheus-scrape-config -oyaml \
     | diff - tmp.prometheus-scrape-config.yaml
   ```

   Or, to just see our changes (without `metadata.annotations`) run:

   ```shell
   CHANGED_LINES=$(echo $(cat config/monitoring/metrics/prometheus/100-prometheus-scrape-kn-eventing.yaml | wc -l) + 1 | bc)
   kubectl get configmap -n  knative-monitoring prometheus-scrape-config -oyaml \
     | diff - tmp.prometheus-scrape-config.yaml \
     | head -n $CHANGED_LINES
   ```

3. Restart Prometheus

   To pick up this new config, the pods that run Prometheus need to be
   restarted, run:

   ```shell
   kubectl delete pods -n knative-monitoring prometheus-system-0 prometheus-system-1
   ```

   And they will come back:

   ```shell
   $ kubectl get pods -n knative-monitoring
   NAME                                 READY   STATUS    RESTARTS   AGE
   grafana-d7478555c-8qgf7              1/1     Running   0          22h
   kube-state-metrics-765d876c6-z7dfn   4/4     Running   0          22h
   node-exporter-5m9cz                  2/2     Running   0          22h
   node-exporter-z59gz                  2/2     Running   0          22h
   prometheus-system-0                  1/1     Running   0          32s
   prometheus-system-1                  1/1     Running   0          36s
   ```

4. To remove the temp files, run:

   ```shell
   rm tmp.prometheus-scrape-kn-eventing.yaml
   rm tmp.prometheus-scrape-config.yaml
   ```

#### Remove Scrape Config

Remove the text related to Knative Eventing from `prometheus-scrape-config`,

```shell
kubectl edit configmap -n  knative-monitoring prometheus-scrape-config
```

And then restart Prometheus.

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


## StackDriver Collection

1.  Install Knative Stackdriver components by running the following command from
    the root directory of [knative/serving](https://github.com/knative/serving)
    repository:

    ```shell
      kubectl apply --recursive --filename config/monitoring/100-namespace.yaml \
          --filename config/monitoring/metrics/stackdriver
    ```

1. Run the following command to setup StackDriver as the metrics backend:

   ```
   kubectl edit cm -n knative-eventing config-observability
   ```

   Add `metrics.backend-destination: stackdriver` and `metrics.allow-stackdriver-custom-metrics: "true"`
    to the `data` field. You can find detailed information in `data._example` field in the
   `ConfigMap` you are editing.
1. Open the StackDriver UI and see your resource metrics in the StackDriver Metrics Explorer.
 You should be able to see metrics with the prefix `custom.googleapis.com/knative.dev/`.
