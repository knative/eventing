# Metrics

This is a list of metrics exported by Knative Eventing components.

## Broker ingress

| Name | Type | Description | Tags
| ---- | ---- | ---- | ---- |
| `messages_total` | count | Number of messages received. | `result`, `broker` |
| `dispatch_time` | histogram | Time to dispatch a message. | `result`, `broker` |
