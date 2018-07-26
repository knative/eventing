# Resource Groups

Knative Eventing API is grouped into _Channels_, _Feeds_ and _Flows_:

* _Channels_ define an abstraction between the eventing infrastructure and the
  consumption and production of the events.

* _Feeds_ bridge the event source into the eventing framework.

* _Flows_ abstract the path of an event from a source takes to reach an event
  consumer.

Typically, _Feeds_ perform out-of-cluster provisioning, while _Channels_ are
within-cluster management of event delivery. _Flows_ is a higher order wrapper
used to connect event producers directly with event consumers leveraging 
_Feeds_ and _Channels_.

# Resource Types

The primary resources in the Knative Eventing _Feeds_ API are EventSource,
EventType and Feed:

* **EventSource**, an archetype of an event producer.

* **EventType**, the schema for an event.

* **Feed**, the association between the output of an event producer to the
  input of an event consumer.


The primary resources in the Knative Eventing _Channels_ API are Channel,
Subscription and Bus:

* **Channel**, a named endpoint which accepts and forwards events.

* **Bus**, an implementation of an event delivery mechanism.

* **Subscription**, an expressed interest in events to be delivered to a
  service.

The primary resources in the Knative Eventing _Flows_ API are Flow:

* **Flow**, the connection between an event producer to the event consumer.

![Object Model](images/overview-reference.png)

See the [Knative Eventing Docs
Architecture](https://github.com/knative/docs/blob/master/eventing/README.md#architecture)
for more details.


