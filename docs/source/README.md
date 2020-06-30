# Event Sources

## Specification

This [document](../spec/sources.md) contains the source specification.

## Implementations

There are various ways to implement Event Sources, as described in the
[documentation](https://knative.dev/docs/eventing/samples/writing-receive-adapter-source/).

### Receive Adapters

Receive adapter is a common pattern for implementing sources. See the
[documentation](https://knative.dev/docs/eventing/samples/writing-receive-adapter-source/)
for more details.

#### Multi tenancy

Single-tenant receive adapters handle one source instance (CR) at a time, i.e.
there is a one-to-one mapping between the receive adapter, and the source
instance specification. Multi-tenant receive adapters can handle more than one
source instance at a time, either all CRs in one namespace or all CRs in the
cluster.

While single-tenant receive adapters work well for high volume, always "on"
event sources, it is not optimal (i.e. waste precious resources, cpu and memory)
for sources producing "few" events, such as PingSource, APIServerSource and
GitHubSource, or for small applications. On the other hand, single-tenant
receive adapters are fairly easy to implement and more transparent compare to
their multi-tenant equivalent.

#### Installation

Event Source might provide both single-tenant and multi-tenant receive adapters.
For those event sources, multiple installation artifacts are provided.
