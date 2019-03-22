# Registry Proposal

## Objective

Design an initial version of the Registry that can support discoverability of 
the different event types that can be consumed from the eventing mesh.

## Requirements

Our design revolves around the following core requirements:

1. We should have a Registry per namespace to enforce isolation.
2. The Registry should contain only the event types that can be consumed from
the eventing mesh.
3. The event types stored in the Registry should contain all the required information 
for a consumer to create a trigger without resorting to some other OOB mechanism.

## Design Ideas

### EventType CRD

We propose having a namespaced EventType CRD. Here is an example of how a CR would look like:

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: EventType
metadata:
  name: repopush
spec:
  type: repo:push
  source: user/repo
  schema: http://schemas/repo/push
  broker: default
```


- The `name` of the EventType is advisory, non-authoritative. Given that Cloud Event types can 
contain characters that may not comply with Kubernetes naming conventions, we will (slightly) 
modify those names to make them K8s-compliant, whenever we need to generate them. 

- `type` is authoritative. This refers to the Cloud Event type as it enters into the eventing mesh. 

- `source`: an identifier of where we receive the event from. This might not necessarily be the Cloud Event source 
attribute.

- `schema` is a URI with the EventType schema. It may be a JSON schema, a protobuf schema, etc. It is optional.

- `broker` refers to the Broker that can provide the EventType. 

### EventType Instantiation

We foresee the following ways of populating the Registry:

**1. Event Source CR installation**

Upon installation of an Event Source CR, the source will register its EventTypes.

Example:

```
apiVersion: sources.eventing.knative.dev/v1alpha1
kind: GitHubSource
metadata:
  name: github-source-sample
spec:
  eventTypes:
    - push
    - pull_request
  ownerAndRepository: linda/eventing
  accessToken:
    secretKeyRef:
      name: github-secret
      key: accessToken
  secretToken:
    secretKeyRef:
      name: github-secret
      key: secretToken
  sink:
    apiVersion: eventing.knative.dev/v1alpha1
    kind: Broker
    name: default

```
 
By applying this file, two EventTypes will be registered, with types `push` and `pull_request`, 
source `linda/eventing`, for the `default` Broker in the `default` namespace.

**2. Manual User Registration**

A user manually `kubectl applies` an EventType CR.

Example: 

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: EventType
metadata:
  name: repofork
spec:
  type: repo:fork
  source: user/repo
  broker: dev
``` 

This would register the EventType named `repofork` with type `repo:fork`, source `user/repo` in the `dev` 
Broker of the `default` namespace.


**3. Broker Auto Registration Policy**

Upon arrival of a non-registered EventType to a Broker ingress, and in case the Broker 
ingress policy allows auto-registration of EventTypes, the Broker will create the EventType.

Example: 

Set up a Broker with `autoAdd` enabled.

```
apiVersion: eventing.knative.dev/v1alpha1
kind: Broker
metadata:
  name: auto-add-demo
spec:
  ingressPolicy:
    autoAdd: true
```

Now if someone emits a non-registered event (e.g., `dev.knative.foo.bar`)
into the Broker `auto-add-demo`, an EventType will be created upon the event arrival. 
Note that the Broker's address is well-known, it will always be
`<name>-broker.<namespace>.svc.<ending>`. In this case case, it is
`auto-add-demo-broker.default.svc.cluster.local`.

We can send the event manually. While SSHed into a `Pod` with the Istio sidecar, run:

```shell
curl -v "http://auto-add-demo-broker.default.svc.cluster.local/" \
  -X POST \
  -H "X-B3-Flags: 1" \
  -H "CE-CloudEventsVersion: 0.1" \
  -H "CE-EventType: dev.knative.foo.bar" \
  -H "CE-EventTime: 2018-04-05T03:56:24Z" \
  -H "CE-EventID: 45a8b444-3213-4758-be3f-540bf93f85ff" \
  -H "CE-Source: dev.knative.example" \
  -H 'Content-Type: application/json' \
  -d '{ "much": "wow" }'
```
 
The Broker `auto-add-demo` will then create the EventType with type `dev.knative.foo.bar` in the Registry.

## Use Case: Discoverability

*As an Event Consumer I want to discover the different event types that I can consume from a 
particular broker without resorting to any OOB mechanism.*

By adding the spec.* fields of EventType as custom columns in the CRD we can fulfill this use case.


`$ kubectl get eventtypes -n default`


NAME | TYPE | SOURCE | SCHEMA | BROKER | READY | REASON
--- | --- | --- | --- | --- | --- | ---
dev.knative.foo.bar-55wcn | dev.knative.foo.bar | dev.knative.example | | auto-add-demo |   True | |
repofork | repo:fork | user/repo | |  dev | False | BrokerIsNotReady |
repopush | repo:push | user/repo | http://schemas/repo/push |  default | True | | 
dev.knative.source.github.push-34cnb | dev.knative.source.github.push | linda/eventing | | default | True | |
dev.knative.source.github.pullrequest-86jhv | dev.knative.source.github.pull_request | linda/eventing | | default | True | | 

