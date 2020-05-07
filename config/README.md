# Welcome to the knative/eventing config directory!

The files in this directory are organized as follows:

- `core/`: the elements that are required for knative/eventing to function,
- `channels/`: reference implementations of the Channel abstraction,
- `brokers/`: reference implementations of Broker abstraction,
- `monitoring/`: an installable bundle of tooling for assorted observability
  functions,
- `*.yaml`: symlinks that form a particular "rendered view" of the
  knative/eventing configuration.

## Core

The Core is complex enough that it further breaks down as follows:

- `roles/`: The [cluster] roles needed for the core controllers to function, or
  to plug knative/eventing into standard Kubernetes RBAC constructs.
- `configmaps/`: The configmaps that are used to configure the core components.
- `resources/`: The eventing resource definitions.
- `webhooks/`: The eventing {mutating, validating} admission webhook
  configurations, and supporting resources.
- `deployments/`: The eventing executable components and associated
  configuration resources.
