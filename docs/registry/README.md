# Registry Proposal

## Objective

Design an **initial** version of the Registry that can support discoverability of 
the different event types that can be consumed from the eventing mesh.

## Requirements

Our design revolves around the following core requirements:

1. We should have a Registry per namespace to enforce isolation.
2. The Registry should contain the event types that can be consumed from
the eventing mesh. If an event type is not ready for consumption, we should explicitly indicate so.
3. The event types stored in the Registry should contain all the required information 
for a consumer to create a Trigger without resorting to some other OOB mechanism.

## Design Ideas

### EventType CRD

We propose having a namespaced EventType CRD. Here is an example of how a CR would look like:

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: EventType
metadata:
  name: repopush
  namespace: default
spec:
  type: repo:push
  source: my-user/my-repo
  schema: /my-schema
  broker: default
```


- The `name` of the EventType is advisory, non-authoritative. Given that Cloud Event types can 
contain characters that may not comply with Kubernetes naming conventions, we will (slightly) 
modify those names to make them K8s-compliant, whenever we need to generate them. 

- `type` is authoritative. This refers to the Cloud Event type as it enters into the eventing mesh. 

- `source`: an identifier of where we receive the event from. This might not necessarily be the Cloud Event source 
attribute. 

If we have control over the entity emitting the Cloud Event, as is the case of many of our receive adaptors, 
then we propose to add a Cloud Event custom extension (e.g., from) with this information, to ease the creation of filters 
on Triggers later on.
As the Cloud Event source attribute is somewhat useless (e.g., github pull requests are populated with `https://github.com/<owner>/<repo>/pull/<pull_id>`), 
there is no way of doing exact matching of Cloud Event sources on Triggers. Thus, we propose adding this custom extension 
to Cloud Events whenever we can. If the extension is not present, then we fallback to the Cloud Event source. 
Note that when we start supporting more advanced filtering mechanisms on Triggers, we might not need this.


- `schema` is a URI with the EventType schema. It may be a JSON schema, a protobuf schema, etc. It is optional.

- `broker` refers to the Broker that can provide the EventType. 

### EventType Instantiation

We foresee the following ways of populating the Registry:

**1. Event Source CR installation**

Upon installation of an Event Source CR, the source will register its EventTypes.

Example:

```yaml
apiVersion: sources.eventing.knative.dev/v1alpha1
kind: GitHubSource
metadata:
  name: github-source-sample
  namespace: default
spec:
  eventTypes:
    - push
    - pull_request
  ownerAndRepository: my-other-user/my-other-repo
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
 
By applying the above file, two EventTypes will be registered, with types `dev.knative.source.github.push` and 
`dev.knative.source.github.pull_request`, source `my-other-user/my-other-repo`, for the `default` Broker in the `default`
 namespace, and with owner `github-source-sample`.
 
In YAML, the EventTypes would look something like these:

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: EventType
metadata:
  generateName: dev.knative.source.github.push-
  namespace: default
  owner: # Owned by github-source-sample
spec:
  type: dev.knative.source.github.push
  source: my-other-user/my-other-repo
  broker: default
---
apiVersion: eventing.knative.dev/v1alpha1
kind: EventType
metadata:
  generateName: dev.knative.source.github.pullrequest-
  namespace: default
  owner: # Owned by github-source-sample
spec:
  type: dev.knative.source.github.pull_request
  source: my-other-user/my-other-repo
  broker: default
```

Two things to notice: 
- We generate the names by stripping invalid characters from the original type (e.g., `_`)
- The `spec.type` adds the prefix `dev.knative.source.github.` This is a separate discussion on whether we should 
change the (GitHub) types or not.

**2. Manual User Registration**

A user manually `kubectl applies` an EventType CR.

Example: 

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: EventType
metadata:
  name: repofork
  namespace: default
spec:
  type: repo:fork
  source: my-other-user/my-other-repo
  broker: dev
``` 

This would register the EventType named `repofork` with type `repo:fork`, source `my-other-user/my-other-repo` 
in the `dev` Broker of the `default` namespace.


**3. Broker Auto Registration Policy**

Upon arrival of a non-registered EventType to a Broker ingress, and in case the Broker 
ingress policy allows auto-registration of EventTypes, the Broker will create the EventType.
Note that the creation of the EventType is done asynchronously, i.e., the Cloud Event is accepted and sent 
to the appropriate Trigger(s) in parallel of the EventType creation. If the creation fails, on a subsequent arrival 
there will be a new creation attempt.  


Example: 

Set up a Broker with `autoAdd` enabled.

```yaml
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
In YAML, the EventType would look something like these:

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: EventType
metadata:
  generateName: dev.knative.foo.bar-
  namespace: default
  owner: # Owned by auto-add-demo?
spec:
  type: dev.knative.foo.bar
  source: dev.knative.example
  broker: auto-add-demo
```

## Discoverability Use Case

By adding the spec.* fields of EventType as custom columns in the CRD we can fulfill the discoverability 
use case: 

*As an Event Consumer I want to be able to discover the different event types that I can consume 
from the different Brokers, without resorting to any OOB mechanism.*

First, Event Consumers will list the EventTypes registered in the system

`$ kubectl get eventtypes -n default`

```
NAME                                         TYPE                                    SOURCE                       SCHEMA      BROKER          READY  REASON
dev.knative.foo.bar-55wcn                    dev.knative.foo.bar                     dev.knative.example                      auto-add-demo   True 
repofork                                     repo:fork                               my-other-user/my-other-repo              dev             False  BrokerIsNotReady
repopush                                     repo:push                               my-other-user/my-other-repo  /my-schema  default         True 
dev.knative.source.github.push-34cnb         dev.knative.source.github.push          my-user/my-repo                          default         True 
dev.knative.source.github.pullrequest-86jhv  dev.knative.source.github.pull_request  my-user/my-repo                          default         True  
```

Then, they will be able to *easily* create the appropriate Trigger(s)


```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Trigger
metadata:
  name: my-service-trigger
  namespace: default
spec:
  filter:
    sourceAndType:
      type: dev.knative.foo.bar
      source: dev.knative.example
  subscriber:
    ref:
     apiVersion: serving.knative.dev/v1alpha1
     kind: Service
     name: my-service
```

## Broker Ingress Policies

We propose adding an `ingressPolicy` field to the Broker's spec CRD. 
Here are three CR examples with different policies. Note that we are omitting 
Broker's fields irrelevant for this discussion.

**1. Allow Any**

By not specifying an ingress policy, the default will be to accept any event.

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Broker
metadata:
  name: broker-allow-any
```  

**2. Allow Registered**

By setting the ingress policy to not allow any, the broker will accept only events with EventTypes in the Registry.

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Broker
metadata:
  name: broker-allow-registered
spec:
  ingressPolicy:
    allowAny: false
```    

**3. Auto Add**

By setting the ingress policy to auto add, the broker will accept any event and will add its EventType 
to the Registry (in case is not present).

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Broker
metadata:
  name: broker-auto-add
spec:
  ingressPolicy:
    autoAdd: true 
```
