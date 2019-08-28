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

| Name                               | Type      | Description                                          | Tags                                           |
| ---------------------------------- | --------- | ---------------------------------------------------- | ---------------------------------------------- |
| `trigger_events_total`             | count     | Number of events received.                           | `result`, `broker`, `trigger`                  |
| `trigger_dispatch_time`            | histogram | Time to dispatch an event.                           | `result`, `broker`, `trigger`                  |
| `trigger_filter_time`              | histogram | Time to filter an event.                             | `result`, `broker`, `trigger`, `filter_result` |
| `broker_to_function_delivery_time` | histogram | Time from ingress of an event until it is dispatched | `result`, `broker`, `trigger`                  |

## Access metrics

Accessing metrics requires Prometheus and Grafana installed. Follow the
[instructions to install Prometheus and Grafana](https://github.com/knative/docs/blob/master/docs/serving/installing-logging-metrics-traces.md)
in namespace `knative-monitoring`.

## Prometheus Collection

> _All commands assume root of repo._

1. Enable Knatives install of Prometheus to scrape Knative with GCP, run the
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

Remove the text related to Cloud Run Events from `prometheus-scrape-config`,

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
