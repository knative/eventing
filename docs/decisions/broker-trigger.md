# Broker and Trigger model (for 0.5.0)

Decision date: 6 March 2019

## Define High-Level Objects Related to Eventing

The goal of eventing is to provide an abstract "bucket of events" which event
consumers can sample from using filters on CloudEvent
[context attributes](https://github.com/cloudevents/spec/blob/master/spec.md#context-attributes).
Event producers can deliver events into the bucket without needing to understand
the exact mechanics of routing to specific consumers. Event producers and
consumers are decoupled temporally but also through a deliberately black-box
abstraction which can optimize the underlying routing layer dynamically.

## Definitions

- **Event Producer**: a system which creates events based on occurrences. E.g.
  GitHub (the hosted service).
- **Event Consumer**: a destination which receives events. E.g. the application
  processing an event. Typically, this is the "sink" in the processing flow of
  an Event, but could be a data processing system like Spark.
- **Sink**: a generic term to mean the destination for an event being sent from
  a component. Synonym for Event Consumer.
- **Event Source**: a helper object that can be used to connect an Event
  Producer to the Knative eventing system by routing events to an Event
  Consumer. E.g. the
  [GitHub Knative Source](https://github.com/knative/eventing-sources/tree/master/pkg/reconciler/githubsource)
- **Receive Adapter**: a data plane entity (Knative Service, Deployment, etc)
  which performs the event routing from outside the Knative cluster to the
  Knative eventing system. Receive Adapters are created by Event Sources to
  perform the actual event routing.

## Objects

The following objects are the minimum set needed to experiment and collect
customer feedback on the Eventing object model, aka an MVP (including rough YAML
definitions; final naming and field definitions subject to change based on
implementation experience).

### Broker

A Broker represents a "bucket of events". It is a namespaced object, with
automation provided to provision a Broker named `default` in
appropriately-labelled namespaces. It is expected that most users will not need
to explicitly create a Broker and simply enabling eventing in the namespace to
create the `default` Broker will be sufficient. (The `default` broker will be
provisioned automatically by a controller upon appropriate namespace annotation.
This controller will _reconcile_ on the annotation, and so will recreate the
`default` Broker if it is deleted. Removing the annotation will be the correct
way to remove the reconciled Broker.) Assuming that the Broker model is
successful, future recommendations for Source CRD authors would be to default
the Source's `spec.sink` attribute to point to the `default` Broker in the
namespace.

Historical storage and replay of events is not part of the Broker design or MVP.

Broker implements the "Addressable" duck type to allow Source custom resources
to target the Broker. Broker DNS names should also be predictable (e.g.
`default-broker` for the `default` Broker), to enable whitebox event producers
to target the Broker without needing to reconcile against the "Addressable" duck
type.

#### Broker Object definition

```golang
type Broker struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec BrokerSpec `json:"spec,omitempty"`
	Status BrokerStatus `json:"status,omitempty"`
}

type BrokerSpec struct {
	// Empty for now
}

type BrokerStatus struct {
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	Address duckv1alpha1.Addressable `json:"address,omitempty"`
}
```

### Trigger

A Trigger represents a registration of interest in a filtered selection of
events delivered to a Broker which should be forwarded to an Addressable Object
or a URL. For the MVP, Triggers may only target Addressable objects in the same
namespace.

#### Buffering

Triggers are assumed to buffer (durably queue) filtered events from the Broker
from the time the Trigger is created, and to deliver the buffered events to the
target Addressable as it is able to receive them. Delivery to the target
Addressable should be at least once, though backing storage implementations may
set limits on actual retries and durability.

#### Filtering

An event consumer which is interested in several sets of events may need to
create multiple Triggers, each with an ANDed set of filter conditions which
select the events. Trigger selection criteria are independent, not aggregated in
the Broker. In the case of multiple Triggers with the same target whose filter
conditions select an overlapping set of events, each Trigger which selects an
incoming event will result in a separate delivery of the event to the target. If
multiple delivery is undesirable, developers should take care to not create
overlapping filter conditions.

At some later point, we expect that a more complex set of filtering expressions
will be supported. For the initial MVP, all match or exact match on the
CloudEvents
[`type`](https://github.com/cloudevents/spec/blob/master/spec.md#type) and
[`source`](https://github.com/cloudevents/spec/blob/master/spec.md#source)
attributes will be supported.

#### Trigger Object Definition

```golang
type Trigger struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec TriggerSpec `json:"spec,omitempty"`
	Status TriggerStatus `json:"status,omitempty"`
}

type TriggerSpec struct {
	// Defaults to 'default'.
	Broker string `json:"broker,omitempty"`
	Filter *TriggerFilter `json:"filter,omitempty"`
	// As from Subscription's `spec.subscriber`
	Target *SubscriberSpec `json:"target,omitempty"`
}

type TriggerFilter struct {
	// Exact match expressions; if empty, all values are matched.
	Type   string `json:"type,omitempty"`
	Source string `json:"source,omitempty"`
}

type TriggerStatus struct {
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	SubscriberURI string `json:"subscriberURI,omitempty"`
}
```

## Backing documents

[2019-02-22 Decision Making -- UX targets](https://docs.google.com/spreadsheets/d/16aOhfRnkaGcQIOR5kiumld-GmrgGBIm9fppvAXx3mgc/edit)

Accepted decisions (2+ votes)

- Have a "Broker" object
  - Default sink on sources
  - Enable with namespace/default Broker
- Have a "Trigger" object
  - "filter" subfield (not top-level filters)
  - "filter" has exact type match
  - "filter" has exact source match
  - target is same-namespace
- Registry requirements are not clear enough to include in this decision.
  - Future user stories could change this and motivate a particular
    implementation.

[UX Proposals Comparision](https://docs.google.com/document/d/1fRpM4u4mP2fGUBmScKQ9_e77rKz_7xh_Thwxp8QXhUA/edit#)
(includes background and discussion)
