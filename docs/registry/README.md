# Registry Proposal

## Problem 

As an `Event Consumer` I want to be able to discover the different event types that I can consume 
from the different Brokers, without resorting to any OOB mechanism. 
This is also known as the `discoverability` use case, and is the main focus of this proposal. 

## Objective

Design an **initial** version of the **Registry** for the **MVP** that can support discoverability of 
the different event types that can be consumed from the eventing mesh. For details on the different user stories 
that this proposal touches, please refer to the 
[User stories and personas for Knative eventing](https://docs.google.com/document/d/15uhyqQvaomxRX2u8s0i6CNhA86BQTNztkdsLUnPmvv4/edit?usp=sharing) document.

#### Out of scope

- Registry to Registry communication. This doesn't seem needed for an MVP.
- Security-related matters. Those are handled offline by the `Cluster Configurator`, e.g., the `Cluster Configurator` 
takes care of setting up Secrets for connecting to a GitHub repo, and so on.
- Registry synchronization with `Event Producers`. We assume that if new GitHub events are created by 
GitHub after our cluster has been configured (i.e., our GitHub CRD Source installed) and the appropriate webhooks 
have been created, we will need to create new webhooks (and update the GitHub CRD Source) if we want to listen for 
those new events. Until doing so, those new events shouldn't be listed in the Registry. 
Such task will again be in charge of the `Cluster Configurator`.

## Requirements

Our design revolves around the following core requirements:

1. We should have a Registry per namespace to enforce isolation.
2. The Registry should contain the event types that can be consumed from
the eventing mesh in order to support the `discoverability` use case.
If an event type is not ready for consumption, we should explicitly indicate so (e.g., if the Broker 
is not ready).
3. The event types stored in the Registry should contain all the required information 
for a consumer to create a Trigger without resorting to some other OOB mechanism.


## Proposed Ideas

### EventType CRD

We propose introducing a namespaced-EventType CRD. Here is an example of how a CR would look like:

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

### Typical Flow

`1.` A `Cluster Configurator` configures the cluster in a way that allows the population of EventTypes in the Registry. 
We foresee the following three ways of populating the Registry so far:

`1.1` Event Source CR installation

Upon installation of an Event Source CR by a `Cluster Configurator`, the Source will register its EventTypes.

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
 
Note that the `Cluster Configurator` is the person in charge of taking care of authentication-related matters. E.g., if a new `Event Consumer` 
wants to listen for events from a different GitHub repo, the `Cluster Configurator` will take care of the necessary secrets generation, 
and new Source instantiation.
 
In YAML, the above EventTypes would look something like these:

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
- The `spec.type` adds the prefix `dev.knative.source.github.` This is a **separate discussion** on whether we should 
change the (GitHub) types or not.

`1.2.` Manual Registration

The `Cluster Configurator` manually `kubectl applies` an EventType CR.

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

As under the hood, `kubeclt apply` just makes a REST call to the API server with the appropriate RBAC permissions, 
the `Cluster Configurator` can give EventType `create` permissions to trusted parties, so that they can register 
their EventTypes.  


`1.3` Broker Auto Registration Policy

The `Cluster Configurator` configures the Broker ingress policy to allow auto-registration of EventTypes. 
Upon arrival of a non-registered EventType to the Broker ingress, the Broker will then create that type.
Note that the creation of the EventType is done asynchronously, i.e., the Cloud Event is accepted and sent 
to the appropriate Trigger(s) in parallel of the EventType creation. If the creation fails, on a subsequent arrival 
there will be a new creation attempt.

Although sub-optimal (as `Event Consumers` find out about EventTypes upon first arrival), we believe this mechanism is needed 
for Sources where the `Cluster Configurator` does not know in advance the EventTypes they can produced (e.g., a ContainerSource). 

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

`2.` An `Event Consumer` checks the Registry to see what EventTypes it can consume from the mesh.

Example:

`$ kubectl get eventtypes -n default`

```
NAME                                         TYPE                                    SOURCE                       SCHEMA      BROKER          READY  REASON
dev.knative.foo.bar-55wcn                    dev.knative.foo.bar                     dev.knative.example                      auto-add-demo   True 
repofork                                     repo:fork                               my-other-user/my-other-repo              dev             False  BrokerIsNotReady
repopush                                     repo:push                               my-other-user/my-other-repo  /my-schema  default         True 
dev.knative.source.github.push-34cnb         dev.knative.source.github.push          my-user/my-repo                          default         True 
dev.knative.source.github.pullrequest-86jhv  dev.knative.source.github.pull_request  my-user/my-repo                          default         True  
```

`3.` The `Event Consumer` creates a Trigger to listen to an EventType in the Registry. 

Example:

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

### Broker Ingress Policies

We briefly mentioned configuring an ingress policy in the Broker so that it can auto-register EventTypes. 
In order to support that, we propose adding an `ingressPolicy` field to the Broker's spec CRD.
 
Below are three CR examples with Brokers configured using different policies. 
We are omitting Broker's fields irrelevant to this discussion. 
Note that such configuration is done by the `Cluster Configurator`.

- Allow Any

By not specifying an ingress policy, the default will be to accept any event.

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Broker
metadata:
  name: broker-allow-any
```  

- Allow Registered

By setting the ingress policy to not allow any, the Broker will accept only events with EventTypes in the Registry.

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Broker
metadata:
  name: broker-allow-registered
spec:
  ingressPolicy:
    allowAny: false
```    

- Auto Add

By setting the ingress policy to auto add, the Broker will accept any event and will add its EventType 
to the Registry (in case it is not present).

```yaml
apiVersion: eventing.knative.dev/v1alpha1
kind: Broker
metadata:
  name: broker-auto-add
spec:
  ingressPolicy:
    autoAdd: true 
```

Note that more policies should probably need to be configured, e.g., allow auto-registration of EventTypes received 
from within the cluster (e.g., from a response of a Service running in the cluster) as opposed to external services, and so on. 
We just enumerate three simple cases here.

## FAQ

Here is a list of frequently asked questions that may help clarify the scope of the Registry.

- Is the Registry meant to be used just for creating Triggers or for also setting up Event Sources?

    It's mainly intended for helping the `Event Consumer` with the `discoverability` use case. Therefore, 
    it is meant for helping out creating Triggers.

- If I have a simple use case where I'm just setting up an Event Source and my KnService is its Sink (i.e., no Triggers involved), 
is there a need/use for the Registry?

    In this case, we believe there is no need for the Registry. As you can see in the EventType CRD, there is a mandatory 
    `broker` field. If you are not sending events to a Broker, then there is no need to use a Registry. 
    Implementation-wise, we can check whether the Source's sink kind is `Broker`, and if so, then register its EventTypes.   

- Is the Registry meant to be used in a single-user environment where the same person is setting up both the Event Source and 
the destination Sink?

    We believe is mainly intended for multi-user environment. A `Cluster Configurator` persona is in charge of setting up 
    the Sources, and `Event Consumers` are the ones that create the Triggers. Having said that, it can also be used in 
    a single-user environment, but the Registry might not add much value compared to what we have right now in terms of 
    `discoverability`, but it surely does in terms of, for example, `admission control`. 
    
- Does a user need to know which type of environment they're in before they should know if they should look at the Registry? 
In other words, is a Registry always going to be there and if not under what conditions will it be?

    If there are Sources pointing to Brokers, then there should be a Registry. `Event Consumers` will always be able to 
    `kubectl get eventtypes -n <namespace>`.
    
- Once an Event Source is created, how is a new one created with different auth in an env where the user is really just meant 
to deal with Triggers? This may not be a Registry specific question but if one of the goals of the Registry is to make it 
so that the user only deals with Triggers using the info in the Registry, I think this aspect comes into play.

    We believe the Event Source instantiation with different credentials should be handled by the `Cluster Configurer`. If the 
    `Cluster Configurer` persona happens to be the same person as the `Event Consumer` persona, then it will have to take care 
    of creating the Source. This is related to the question of a single-user, multi-user environment above.
    
- I've heard conflicting messages around whether the Registry is just a list of Event Types or it will also be a list of 
Event Sources so that the user doesn't need to query the CRDs to get the list. We need to be clear about this.

    The Registry is a list of EventTypes. Having said that, the `Event Consumer` could also (if it has the proper RBAC permissions) 
    list Event Sources (e.g., `kubectl get crds -l eventing.knative.dev/source=true`), but that list is not part of what we call 
    Registry here. The idea behind the fields in the EventType CRD is to have all the necessary information there in 
    order to create a Triggers, thus, in most cases, the `Event Consumer` shouldn't have to list Sources. 

-  I wonder if the Event Source populating the Registry should happen when the Event Source is loaded into the system, 
meaning when the Event Source's CRD is installed (not when an instance of the CRD is created). 

The problem with that is that you don't have a namespace (nor Broker, user/repo, etc.) at that point. 
Which namespace the EventType should be created on? Pointing to which Broker? 
Implementation-wise, one potential solution is to have a controller for source CRDs, whenever one is installed, search for all the namespaces with 
eventing enabled (`kubectl get namespaces -l knative-eventing-injection=enabled`), and adding all the possible EventTypes from that CRD to each of 
the Brokers in those namespaces. A downside of this is that the Registry information is not "accurate", in the sense that it only has info about EventTypes 
that may eventually flow in the system. But actually, they will only be able to flow when a CR is created.

- ...

