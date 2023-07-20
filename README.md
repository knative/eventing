# Knative Eventing - The Event-driven application platform for Kubernetes

_Secure event processing and discovery with CloudEvents_

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/knative.dev/eventing)
[![Go Report Card](https://goreportcard.com/badge/knative/eventing)](https://goreportcard.com/report/knative/eventing)
[![Releases](https://img.shields.io/github/release-pre/knative/eventing.svg)](https://github.com/knative/eventing/releases)
[![LICENSE](https://img.shields.io/github/license/knative/eventing.svg)](https://github.com/knative/eventing/blob/main/LICENSE)
[![codecov](https://codecov.io/gh/knative/eventing/branch/main/graph/badge.svg)](https://codecov.io/gh/knative/eventing)
[![TestGrid](https://img.shields.io/badge/testgrid-eventing-informational?logo=data:image/x-icon;base64,AAABAAEAICAAAAEACACoCAAAFgAAACgAAAAgAAAAQAAAAAEACAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAALAAAAFQAAACoAAw0AAAAAVQAGGgAAAABgAAAAgAAKJgAAAACqAA0zAAATTAAAGmYAAB1zAAAmmQAAM8wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACgoKCgoKCAsQEBAQEBANDRAQEBAQEAsPEBAQEBAQAAAKCgoKCgoICxAQEBAQEA0NEBAQEBAQCw8QEBAQEBAAAAoKCgoKCggLEBAQEBAQDQ0QEBAQEBALDxAQEBAQEAAACgoKCgoKCAsQEBAQEBANDRAQEBAQEAsPEBAQEBAQAAAKCgoKCgoICxAQEBAQEA0NEBAQEBAQCw8QEBAQEBAAAAoKCgoKCggLEBAQEBAQDQ0QEBAQEBALDxAQEBAQEAAACAgICAgIBwkPDw8PDw8MDA8PDw8PDwkODw8PDw8PAAALCwsLCwsJBAsLCwsLCwYCAwMDAwMDAQkLCwsLCwsAABAQEBAQEA8LEBAQEBAQDQUKCgoKCgoDDxAQEBAQEAAAEBAQEBAQDwsQEBAQEBANBQoKCgoKCgMPEBAQEBAQAAAQEBAQEBAPCxAQEBAQEA0FCgoKCgoKAw8QEBAQEBAAABAQEBAQEA8LEBAQEBAQDQUKCgoKCgoDDxAQEBAQEAAAEBAQEBAQDwsQEBAQEBANBQoKCgoKCgMPEBAQEBAQAAAQEBAQEBAPCxAQEBAQEA0FCgoKCgoKAw8QEBAQEBAAAA0NDQ0NDQwGDQ0NDQ0NCwMFBQUFBQUCDA0NDQ0NDQAADQ0NDQ0NDAIFBQUFBQUDCw0NDQ0NDQYMDQ0NDQ0NAAAQEBAQEBAPAwoKCgoKCgUNEBAQEBAQCw8QEBAQEBAAABAQEBAQEA8DCgoKCgoKBQ0QEBAQEBALDxAQEBAQEAAAEBAQEBAQDwMKCgoKCgoFDRAQEBAQEAsPEBAQEBAQAAAQEBAQEBAPAwoKCgoKCgUNEBAQEBAQCw8QEBAQEBAAABAQEBAQEA8DCgoKCgoKBQ0QEBAQEBALDxAQEBAQEAAAEBAQEBAQDwMKCgoKCgoFDRAQEBAQEAsPEBAQEBAQAAALCwsLCwsJAQMDAwMDAwIGCwsLCwsLBAkLCwsLCwsAAA8PDw8PDw4JDw8PDw8PDAwPDw8PDw8JBwgICAgICAAAEBAQEBAQDwsQEBAQEBANDRAQEBAQEAsICgoKCgoKAAAQEBAQEBAPCxAQEBAQEA0NEBAQEBAQCwgKCgoKCgoAABAQEBAQEA8LEBAQEBAQDQ0QEBAQEBALCAoKCgoKCgAAEBAQEBAQDwsQEBAQEBANDRAQEBAQEAsICgoKCgoKAAAQEBAQEBAPCxAQEBAQEA0NEBAQEBAQCwgKCgoKCgoAABAQEBAQEA8LEBAQEBAQDQ0QEBAQEBALCAoKCgoKCgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA)](https://testgrid.knative.dev/eventing)
[![Slack](https://img.shields.io/badge/Signup-Knative_Slack-white.svg?logo=slack)](https://slack.cncf.io/)
[![Slack](https://img.shields.io/badge/%23eventing-white.svg?logo=slack&color=522a5e)](https://knative.slack.com/archives/C9JP909F0)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/5913/badge)](https://bestpractices.coreinfrastructure.org/projects/5913)

## What is Knative Eventing?

Knative Eventing is a collection of APIs that enable you to use an [event-driven architecture](https://en.wikipedia.org/wiki/Event-driven_architecture) with your applications. You can use these APIs to create components that route events from event producers (known as sources) to event consumers (known as sinks) that receive events. Sinks can also be configured to respond to HTTP requests by sending a response event.

Knative Eventing is a standalone platform that provides support for various types of workloads, including standard Kubernetes Services and Knative Serving Services.

Knative Eventing uses standard HTTP requests to route events from event producers to event consumers, following the rules set by the [CloudEvents specification](https://cloudevents.io/). This is a standard set up by the CNCF that has wide support for many programming languages, making it easy to create, understand, send, and receive events.

Knative Eventing components are loosely coupled, and can be developed and deployed independently of each other. Any producer can generate events before there are active event consumers that are listening for those events. Any event consumer can express interest in a class of events before there are producers that are creating those events.


## What to expect here?

This repository contains an eventing system that is designed to
address a common need for cloud native development:

1. Services are loosely coupled during development and deployed independently
1. A producer can generate events before a consumer is listening, and a consumer
   can express an interest in an event or class of events that is not yet being
   produced.
1. Services can be connected to create new applications
   - without modifying producer or consumer, and
   - with the ability to select a specific subset of events from a particular
     producer.

## More on Knative Eventing

The high level mission of Knative Eventing is: **Enable asynchronous application
development through event delivery from anywhere.**

For the full mission of Knative Eventing see
[docs/mission.md](./docs/mission.md).

For complete Knative Eventing documentation, see
[Knative eventing](https://www.knative.dev/docs/eventing/) or
[Knative docs](https://www.knative.dev/docs/) to learn about Knative.

If you are interested in contributing, see [CONTRIBUTING.md](./CONTRIBUTING.md),
[DEVELOPMENT.md](./DEVELOPMENT.md) and
[Knative working groups](https://knative.dev/community/contributing/working-groups/working-groups/#eventing).

Interested users should join
[knative-users](https://groups.google.com/forum/#!forum/knative-users).
