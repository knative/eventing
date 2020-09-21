# Receive Adapter

A _receive adapter_ is a data-plane component which either produces
[CloudEvents](https://github.com/cloudevents/spec) or imports events and convert
them to [CloudEvents](https://github.com/cloudevents/spec) and forwards those
events to one or several consumers (aka _sink_), depending upon whether it
supports multiple resources (see below for more information) or not.

## Implementation

Knative Eventing provides a library (later referred as _the library_) for
bootstrapping receive adapter implementations.

The main code should be something like this:

```go
package main

import (
  "knative.dev/eventing/pkg/adapter/youradapter"
  "knative.dev/eventing/pkg/adapter/v2"
)

func main() {
  adapter.Main("pingsource",
    youradapter.NewEnvConfig,
    youradapter.NewAdapter)
}
```

`adapter.Main` is responsible for:

- Parsing the command flags and initializing the environment configuration
  returned by `youradapter.NewEnvConfig`.
- Setting up the logger.
- Setting up the profiler server (when enabled).
- Setting up the metrics server.
- Setting up tracing.
- Setting up a ConfigMap watcher (when enabled, always multiple resources).
- Setting up a CloudEvent client, targeting a single sink, for single resource
  adapter, or otherwise targeting no sink.
- Running in leader election mode (when enabled) for being highly-available and
  scalable (pull-based sources only).

## Push vs pull models

The push model describes the processing mode under which the receive adapter
reacts to incoming requests or events. The pull model is the opposite, the
receive adapter does not wait for incoming requests or events but instead it
fetches, or generates events.

The receive adapter library can be used to implement either type of receive
adapters. For pull-based receive adapters, the library provides a mechanism for
making your adapter highly-available and scalable.

## Multiple resources (aka multi-tenancy)

As described in the [source specification](../spec/sources.md), a source is
configured by creating an instance of the resource described by a CRD (e.g. the
PingSource CRD). The receive adapter library provide some flexibility regarding
how many custom resources are handled by a single receive adapter instance (i.e.
a pod).

### Single resource

Single-resource receive adapters handle one source instance at a time, i.e.
there is a one-to-one mapping between the receive adapter, and the source
instance specification. They are fairly easy to implement and more transparent
compare to their multi-resources equivalent (see below).

The receive adapter library automatically configures the CloudEvent client with
the target defined in the `K_SINK` environment variable when defined, in
conformance with the
[SinkBinding runtime contract](../spec/sources.md#sinkbinding).

### Multiple resources

While single-resource receive adapters work well for high volume, always "on"
event sources, it is not optimal (i.e. waste precious resources, cpu and memory)
for sources producing "few" events, such as `PingSource`, `APIServerSource` and
`GitHubSource`, or for small applications. Multi-resource receive adapters can
handle more than one source instance at a time, either all source instances in
one namespace or all source instances in the cluster.

In order to support multi-resources per receive adapter, you need to enable the
Controller watcher feature, as follows:

```go
func main() {
  ctx := signals.NewContext()
  ctx = adapter.WithController(ctx, youradapter.NewController)
  ctx = adapter.WithConfigMapWatcherEnabled(ctx)
  adapter.MainWithContext(ctx, "yourcomponent",
    youradapter.NewEnvConfig,
    youradapter.NewAdapter)
}
```

The library automatically creates a watcher on the ConfigMap named
`config-yourcomponent`. In order to react to any ConfigMap changes, in your
adapter constructor (`NewAdapter`), you must add a ConfigMap watcher, as
follows:

```go
func NewAdapter(ctx context.Context,
                _ adapter.EnvConfigAccessor,
                ceClient cloudevents.Client) adapter.Adapter {
  youradapter := ...

  cmw := adapter.ConfigMapWatcherFromContext(ctx)
  cmw.Watch("config-yourcomponent", youradapter.updateFromConfigMap)

  return youradapter
}
```

The controller associated to the receive adapter generates the ConfigMap. It
relies on the [persistent store](../../pkg/utils/cache/persisted_store.go)
utility. Add these lines to your controller to enable it.

```go
// Create and start persistent store backed by ConfigMaps
store := eventingcache.NewPersistedStore(
    "yourcomponent",
    kubeclient.Get(ctx),
    system.Namespace(),
    "config-yourcomponent",
    yourcomponentInformer.Informer(),
    yourcomponent.Project)

go store.Run(ctx)
```

## Logging, metrics and tracing

The adapter main code automatically sets up logging, a metrics server,
optionally a profiler server and tracing based on configuration parameters
stored in the environment variables named `K_LOGGING_CONFIG`, `K_METRICS_CONFIG`
and `K_TRACING_CONFIG`, respectively.

The adapter does not watch for configuration changes. This is up to the
controller to watch for changes and to update the adapter accordingly.

## High-availability

### Push model

To make your push-based adapter highly-available, you can deploy it either as a
Kubernetes Service, or as a Knative Service. In both cases you can increase the
number of receive adapter replicas for increase reliability and reducing
downtime.

The [GithubSource](https://github.com/knative-sandbox/eventing-github) is a good
example of a highly-available push-based receive adapter leveraging Knative
Service.

### Pull model

The library supports highly-available receive adapter via the leader election
pattern.

You can use `WithHAEnabled` to enable leader elected receive adapter:

```go
func main() {
  ...
  ctx = adapter.WithHAEnabled(ctx)
  ...
}
```

#### Disabling HA

Similar to Knative controller, HA can be disable by passing the `--disable-ha`
flag on the command line.

For instance this flag can be set on the ping source adapter deployment as
follows:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pingsource-mt-adapter
  namespace: knative-eventing
spec:
  template:
    metadata:
      labels:
        eventing.knative.dev/source: ping-source-controller
        sources.knative.dev/role: adapter
    spec:
      containers:
        - args:
            - --disable-ha
          env: ...
          image: ...
```

## Scalability

### Push model

Push-based receive adapters scalability relies on Kubernetes service or Knative
serving.

### Pull model

Currently, pull-based receive adapters don't scale yet. Stay tuned.
