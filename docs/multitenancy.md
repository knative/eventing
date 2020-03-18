# Multi Tenancy

Knative Eventing generally provides two sets of data-plane components:

- single-tenant: these are components capable of handling one and only one resource.
  For instance both the `APIServerSource` and the `GitHubSource` adapters
  are single-tenant. There is one component per resource and usually the component
  is deployed in the same namespace as the resource it manages.
- multi-tenant: these are components capable of handling more than one resource.
  For example, both the in-memory and the kafka dispatchers support
  multi-tenancy. These components can typically be either deployed in
  the system namespace (eg. `knative-eventing`) or in the same namespace as the
  resources it manages.

## Configuring multi-tenancy

Each component potentially supports three multi-tenancy modes:

- single-tenant
- multi-tenant, one per namespace
- multi-tenant, one per cluster

By default all components operate in multi-tenant, one per cluster mode, unless
the component does not support it. See the table below for more details.

The default mode can be overridden by adding the `eventing.knative.dev/scope`
annotation to a resource. The possible values to:

- `resource` (single-tenant),
- `namespace` (multi-tenant, one per namespace),
- `cluster` (multi-tenant, one per cluster)

For instance, this specification:

```yaml
apiVersion: messaging.knative.dev/v1beta1
kind: InMemoryChannel
metadata:
  name: foo-ns
  namespace: default
  annotations:
    eventing.knative.dev/scope: namespace
```

tells Knative Eventing to use the in-memory dispatcher in the `default`
namespace to manage `foo-ns`.

## Components

This list describes which modes are being supported by the Knative Eventing
data-plane components.

<sup>default</sup> indicates the default mode.

### Channels

| Component | single-tenant | multi-tenant, namespace | multi-tenant, cluster |
|--- |---|---|---|
| In-memory dispatcher | No | Yes | Yes<sup>default</sup> |
| Kafka dispatcher | No | Yes | Yes<sup>default</sup> |
| NATS dispatcher | No | No | Yes<sup>default</sup> |

### Brokers

| Component | single-tenant | multi-tenant, namespace | multi-tenant, cluster |
|--- |---|---|---|
| Channel-based broker | No | Yes<sup>default</sup> | No |

### Sources

| Component | single-tenant | multi-tenant, namespace | multi-tenant, cluster |
|--- |---|---|---|
| PingSource | Yes | No | Yes<sup>default</sup> |
| APIServerSource | Yes<sup>default</sup> | No | No |
| AWSSQSSource | Yes<sup>default</sup> | No | No |
| CamelSource | Yes<sup>default</sup> | No | No |
| CephSource | Yes<sup>default</sup> | No | No |
| CouchDBSource | Yes<sup>default</sup> | No | No |
| GithubSource | Yes<sup>default</sup> | No | No |
| KafkaSource | Yes<sup>default</sup> | No | No |
| PrometheusSource | Yes<sup>default</sup> | No | No |


