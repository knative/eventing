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
create the `default` Broker will be sufficient. Assuming that the Broker model
is successful, future recommendations for Source CRD authors would be to default
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

A Trigger represents a registration of interest (request for delivery to an
Addressable object or URL) in a filtered selection of events delivered to a
Broker. For the MVP, Triggers may only target Addressable objects in the same
namespace.

#### Buffering

Triggers are assumed to buffer (durably queue) filtered events from the Broker
from the time they are created, and to deliver the buffered events to the target
Addressable as it is able to receive them.

#### Filtering

An event consumer which is interested in several sets of events may need to
create multiple Triggers, each with an ANDed set of filter conditions which
select the events. In the case of multiple Triggers with the same target whose
filter conditions select an overlapping set of events, events will be delivered
once for each Trigger which selects the event (i.e. Trigger selection criteria
are independent, not aggregated in the Broker).

At some later point, we expect that a more complex set of filtering expressions
will be supported. For the initial MVP, exact match on the CloudEvents
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
- Registry requirements are not clear enough to include in MVP

[UX Proposals Comparision](https://docs.google.com/document/d/1fRpM4u4mP2fGUBmScKQ9_e77rKz_7xh_Thwxp8QXhUA/edit#)
(includes background and discussion)
