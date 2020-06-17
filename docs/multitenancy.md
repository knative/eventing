# Source Multi Tenancy

## Problem

Single-tenant receive adapters handle only one source instance (CR) at a time,
i.e. there is a one-to-one mapping between the receive adapter, and the source instance
specification. While this architecture works well for high volume, always "on" event
sources, it is not optimal (i.e. waste precious resources, cpu and memory) for sources 
producing "few" events, such as PingSource, APIServerSource and GitHubSource,
or for small applications.

## Solution 

Add support for multi-tenant receive adapters capable of handling
more than one source.

## Multi Tenancy Modes

There are two possible multi-tenancy modes:

- `namespace`: multi-tenant receive adapter, one per namespace
- `cluster`: multi-tenant receive adapter, one per cluster

## Configuration

### Via ConfigMap

To change the multi-tenancy mode, edit the `config-sources` ConfigMap
*before* installing Knative Eventing.


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
    # Multitenancy modes. This may be "namespace" or "cluster". The default is "namespace"
    multitenancy: "namespace"
```


