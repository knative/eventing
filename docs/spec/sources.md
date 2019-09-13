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

There are two 

For an operator or a cluster to discover  

There is a distinction between a Kubernetes cluster that has a source CRD installed
and a custom resource (CR is an instance of a CRD). A cluster that has a source CRD 
installed has the ability to signal to the cluster operator that it is capable of
producing events of a certain type by using the [Event Type Registry](#event-type-registry).



There are some guidelines on implementing sources to allow cluster operators and
tools to dynamically discover and understand source installations.

<!-- TODO: expand on CR state of a Source and those expectations. --?>


## Source CRDs

Sources are more useful if they are discoverable. Knative Sources MUST use a
ducktype label to allow controllers and operators the ability to find which
CRDs are considered to be adhering to the
[Source](https://godoc.org/github.com/knative/pkg/apis/duck/v1#Source) ducktype. 

CRDs that are to be understood as a `source` MUST be labeled:

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  labels:
    duck.knative.dev/source: "true" # <-- required to be a source.
```

<!-- TODO: Let's expand a bit on this. It would be maybe be useful to include the motivation here, for example, so support the Source in the listing (for example kubectl get ... command). So in addition to MUST be labeled, why would be helpful. Understood is just a bit of a vague term here. -->

CRDs SHOULD be added to the `sources` category:

```yaml
spec:
  names:
    categories:
      - sources
```

<!-- TODO: We should mention consistent use of the Conditions and in particular the one Ready condition that we have defined in the apis.ConditionReady and probably add that as a recommended additional Printer Column for consistency? -->

<!-- We should also talk about the validation aspects and encourage proper Description for the fields and so forth also?

### Source Validation

OpenAPI

```yaml
  validation:
    openAPIV3Schema:
```

-->

<!-- TODO(n3wscott,nacho): document the registry reqirements.



### Event Type Registry

To be included in the event type registry:

```yaml
todo
```
-->

## Source RBAC

Knative leverages an aggregated RBAC role to allow for controllers to check the
status of source type resources.

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

<!-- Might be worth it to mention a use case or two as examples? Motivation section if you will :)

### Motivation for Aggregated RBAC

TODO.....

-->

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

## Source Resource Shape

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

<!-- TODO

 TODO: We should probably mention here about the apis.ConditionSet and each Source IMHO should have the apis.ConditionReady as the one way to look at the readiness of the Source.

 -->

For a full definition of `Status` and `SinkURI`, please see
[Status](https://godoc.org/github.com/knative/pkg/apis/duck/v1#Status), and
[URL](https://godoc.org/knative.dev/pkg/apis#URL).
