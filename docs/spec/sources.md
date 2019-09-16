# Sources

A **source** is any resource that generates or imports an event and relays that
event to another resource on the cluster via CloudEvents. Sourcing events is
critical to developing a distributed system that reacts to events.

A Source:

- Represents an off or on-cluster system, service or application that produces
  events to be consumed by a sink.
- Produces or imports CloudEvents.
- Sends CloudEvents to the configured **sink**.

In practice, sources are an abstract concept that allow us to create declarative
configurations through the usage of Custom Resource Definitions (CRDs) extending
Kubernetes. Those CRDs are instantiated by creating an instance of the resource.
It is up to the implementation of the source author to understand the best way
to realize the source application. This could be as 1:1 deployments inside of
Kubernetes per resource, as a single multi-tenant application, or even an
off-cluster implementation; or all combinations in-between.

For Kubernetes Operators, there are two states to sources:

1. A source CRD and controller (if required) have been installed into the
   cluster.

   A cluster with a source CRD installed allows the other operators in the
   cluster to find and discover what is possible to source events from. These
   CRDs define the credentials, input parameters, k8s categories, event types,
   duck types, and aggregated RBAC roles required to interpret the source as the
   source author expects. This topic is expanded upon in the
   [Source CRDs](#source-crds) section.

1. A source custom object has been created in the cluster.

   Once a cluster operator creates an instance of a source and provides the
   necessary credentials, input parameters, and event sink, the source
   controller will realize this into whatever is required for that source for
   the current situation. While this resource is running, a cluster operator
   would like to inspect the resource without needing to be fully aware of the
   implementation. This is done by conforming to the [Source]() ducktype. This
   topic is expanded upon in the [Source Custom Objects](#source-custom-objects)
   section.

The goal of requiring CRD labels and running resource shapes is to aid in
discovery and understanding of potential sources that could be leveraged in the
cluster, and sources that exist and are running in the cluster.

## Source CRDs

Sources are more useful if they are discoverable. Knative Sources MUST use a
ducktype label to allow controllers and operators the ability to find which CRDs
are considered to be adhering to the
[Source](https://godoc.org/github.com/knative/pkg/apis/duck/v1#Source) ducktype.

CRDs that are to be understood as a `source` MUST be labeled:

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  labels:
    duck.knative.dev/source: "true" # <-- required to be a source.
```

Labeling sources in this way allows for cluster operators to list CRDs and
filter on which ones are choosing to adhere to the source ducktype. To make this
query,

```shell
kubectl get crds -l duck.knative.dev/source=true
```

CRDs SHOULD be added to the `sources` category:

```yaml
spec:
  names:
    categories:
      - sources
```

By adding to the sources category, we give an easy way to list running sources
in the cluster with,

```shell
kubectl get sources
```

Source CRDs SHOULD provide additional printer columns to provide useful feedback
to cluster operators. For example, if the resource is long-lived, it would be a
good choice to show the `Ready` status and `Reason`, as well as the `Age` of the
resource.

```yaml
additionalPrinterColumns:
  - name: Ready
    type: string
    JSONPath: '.status.conditions[?(@.type=="Ready")].status'
  - name: Reason
    type: string
    JSONPath: '.status.conditions[?(@.type=="Ready")].reason'
  - name: Age
    type: date
    JSONPath: .metadata.creationTimestamp
```

### Source Validation

Sources MUST implement conditions with a `Ready` condition for long lived
sources, and `Succeeded` for batch style sources.

Sources MUST propagate the `sinkUri` to their status to signal to the cluster
where their events are being sent.

Knative has standardized on the following minimum OpenAPI definition of `status`
for Source CRDs:

```yaml
validation:
  openAPIV3Schema:
    properties:
      status:
        type: object
        properties:
          conditions:
            type: array
            items:
              type: object
              properties:
                lastTransitionTime:
                  type: string
                message:
                  type: string
                reason:
                  type: string
                severity:
                  type: string
                status:
                  type: string
                type:
                  type: string
              required:
                - type
                - status
          sinkUri:
            type: string
```

Please see
[SourceStatus](https://godoc.org/github.com/knative/pkg/apis/duck/v1#SourceStatus)
and [Condition](https://godoc.org/github.com/knative/pkg/apis/#Condition) for
more details.

Sources SHOULD provide OpenAPI validation for the `spec` field. At minimum
sources SHOULD have the following,

```yaml
validation:
  openAPIV3Schema:
    properties:
      spec:
        type: object
        required:
          - sink
        properties:
          sink:
            type: object
            description:
              "Reference to an object that will resolve to a domain name to use
              as the sink."
          ceOverrides:
            type: object
            description:
              "Defines overrides to control modifications of the event sent to
              the sink."
            properties:
              extensions:
                type: object
                description:
                  "Extensions specify what attribute are added or overridden on
                  the outbound event. Each `Extensions` key-value pair are set
                  on the event as an attribute extension independently."
```

### Event Type Registry

The Event Type Registry is a feature that allows for software to find all CRDs
labeled as a `Source` ducktype, and extract the defined
[EventType](https://godoc.org/knative.dev/eventing/pkg/apis/eventing/v1alpha1#EventType)s
that could be emitted from a source instance.

To be included in the event type registry a fake `registry` component is defined
in the `openAPIV3Schema` section inside the CRD. It is not our intention to have
cluster operators create resources with this filled out, it is to signal to
other software some metadata about the runtime/dataplane characteristics of this
type of source.

As an example, here is a snip the registry entry for ApiServerSource, a source
that emits Kubernetes event,

```yaml
  validation:
    openAPIV3Schema:
      properties:
        registry:
          type: object
          description: "Internal information, users should not set this property"
          properties:
            eventTypes:
              type: object
              description: "Event types that ApiServerSource can produce"
              properties:
                addResource:
                  type: object
                  properties:
                    type:
                      type: string
                      pattern: "dev.knative.apiserver.resource.add"
                    schema:
                      type: string
                deleteResource:
                  type: object
                  properties:
                    type:
                      type: string
                      pattern: "dev.knative.apiserver.resource.delete"
                    schema:
                      type: string
                updateResource:
                  type: object
                  properties:
                    type:
                      type: string
                      pattern: "dev.knative.apiserver.resource.update"
                    schema:
                      type: string
                ...
```

## Source RBAC

Sources can by any shape and are expected to be extensions onto Kubernetes. To
prevent cluster operators from re-deploying, or re-creating RBAC for all
accounts that will interact with sources, Knative leverages aggregated RBAC
roles to dynamically update the rights of controllers that are using common
service accounts provided by Knative. This allows Eventing controllers to check
the status of source type resources without being aware of the exact source type
or implementation at compile time.

The
[`source-observer` ClusterRole](../../config/200-source-observer-clusterrole.yaml)
looks like:

```yaml
# Use this aggregated ClusterRole when you need read "Sources".
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: source-observer
aggregationRule:
  clusterRoleSelectors:
    - matchLabels:
        duck.knative.dev/source: "true"
rules: [] # Rules are automatically filled in by the controller manager.
```

And new sources MUST include a ClusterRole as part of installing themselves into
a cluster:

```yaml
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: foos-source-observer
  labels:
    duck.knative.dev/source: "true"
rules:
  - apiGroups:
      - example.com
    resources:
      - foos
      - foos/status
    verbs:
      - get
      - list
      - watch
```

## Source Custom Objects

The minimum definition of the Kubernetes resource shape is defined in the
[Source](https://godoc.org/github.com/knative/pkg/apis/duck/v1#Source) ducktype.

### duck.Spec

The `spec` field is expected to have the following minimum shape:

```go
type SourceSpec struct {
    // Sink is a reference to an object that will resolve to a domain name or a
    // URI directly to use as the sink.
    Sink apisv1alpha1.Destination `json:"sink,omitempty"`

    // CloudEventOverrides defines overrides to control the output format and
    // modifications of the event sent to the sink.
    // +optional
    CloudEventOverrides *CloudEventOverrides `json:"ceOverrides,omitempty"`
}
```

For a full definition of `Sink` and `CloudEventsOverrides`, please see
[Destination](https://godoc.org/knative.dev/pkg/apis/v1alpha1#Destination), and
[CloudEventOverrides](https://godoc.org/github.com/knative/pkg/apis/duck/v1#CloudEventOverrides).

### duck.Status

The `status` field is expected to have the following minimum shape:

```go
type SourceStatus struct {
    // inherits duck/v1 Status, which currently provides:
    // * ObservedGeneration - the 'Generation' of the Service that was last
    //   processed by the controller.
    // * Conditions - the latest available observations of a resource's current
    //   state.
    Status `json:",inline"`

    // SinkURI is the current active sink URI that has been configured for the
    // Source.
    // +optional
    SinkURI *apis.URL `json:"sinkUri,omitempty"`
}
```

For a full definition of `Status` and `SinkURI`, please see
[Status](https://godoc.org/github.com/knative/pkg/apis/duck/v1#Status), and
[URL](https://godoc.org/knative.dev/pkg/apis#URL).
