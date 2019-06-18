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
