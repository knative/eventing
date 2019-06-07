# Knative Eventing Mission Statements

## 10-word version

Enable asynchronous application development through event delivery from
anywhere.

## 25-word version

Knative/Eventing offers a consistent, Kubernetes-based platform for reliable,
secure and scalable asynchronous event delivery from packaged or app-created
event sources to on-cluster and off-cluster consumers.

## Full Mission Statement

The Knative Eventing mission focuses on three areas: Developer Experience, Event
Delivery, and Event Sourcing. The primary focus is Developer experience around
event driven applications; event delivery and sourcing provide infrastructure
which enables the developer experience to scale to complex applications.

By leveraging open standards and infrastructure (such as CloudEvents and
Kubernetes), Knative Eventing provides a robust and extensible ecosystem which
accommodates interoperability between open and closed-source frameworks and
services.

**Developer Experience:** Enable developers to write, maintain, and support
event driven applications by providing a uniform experience when sourcing events
from either their own systems and external systems. Provide a standard
experience around consumption of and creation of events using the CloudEvents
standard.

**Event Delivery:** Provide an implementation of event delivery infrastructure,
accompanied by a well documented specification and conformance tests for that
specification. Enable a pluggable architecture for the event delivery mechanism
so that developers and operators can choose the correct event delivery substrate
for their requirements.

**Event Sourcing:** Provide a specification for how to create event importers as
well as guidelines and tools for event providers to cleanly integrate in a
CloudEvents environment.
