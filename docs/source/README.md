# Event Sources

## Specification

This [document](../spec/sources.md) contains the source specification.

## Implementations

There are various ways to implement Event Sources, as described in the [documentation](https://knative.dev/docs/eventing/samples/writing-receive-adapter-source/).
 
### Receive Adapters

Receive adapter is a common pattern for implementing sources. 

#### Multi tenancy

Single-tenant receive adapters handle one source instance (CR) at a time,
i.e. there is a one-to-one mapping between the receive adapter, and the source instance
specification.  Multi-tenant receive adapters can handle more than one source instance at a time, 
either all CRs in one namespace or all CRs in the cluster.

While single-tenant receive adapters work well for high volume, always "on" event
sources, it is not optimal (i.e. waste precious resources, cpu and memory) for sources 
producing "few" events, such as PingSource, APIServerSource and GitHubSource,
or for small applications. On the other hand, single-tenant receive adapters are
fairly easy to implement and more transparent compare to their multi-tenant equivalent.  

#### Configuration

When both single-tenant and multi-tenant receive adapters are available,
the `config-sources` ConfigMap specifies which one is preferred. 

```yaml 
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-sources
  namespace: knative-eventing
data:
  _example: |
    ################################
    #                              #
    #    EXAMPLE CONFIGURATION     #
    #                              #
    ################################
    # This block is not actually functional configuration,
    # but serves to illustrate the available configuration
    # options and document them in a way that is accessible
    # to users that `kubectl edit` this config map.
    #
    # These sample configuration options may be copied out of
    # this example block and unindented to be in the data block
    # to actually change the configuration.
    #
    # Multitenancy modes. This may be "off", "namespace" or "cluster". The default is "cluster"
    multitenancy: "cluster"
```

There are three possible multi-tenancy preferences:

- `off`: no multi-tenancy, use single-tenant receive adapter.
- `namespace`: multi-tenant receive adapter, one per namespace. Fallback to `off' is not available (per source).
- `cluster`: multi-tenant receive adapter, one per cluster. Fallback to `off' is not available (per source).

Event Source implementation SHOULD at least support the single-tenancy.
