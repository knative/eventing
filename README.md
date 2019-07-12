# Knative Eventing

[![GoDoc](https://godoc.org/github.com/knative/eventing?status.svg)](https://godoc.org/github.com/knative/eventing)
[![Go Report Card](https://goreportcard.com/badge/knative/eventing)](https://goreportcard.com/report/knative/eventing)

This repository contains a work-in-progress eventing system that is designed to
address a common need for cloud native development:

1. Services are loosely coupled during development and deployed independently
1. A producer can generate events before a consumer is listening, and a consumer
   can express an interest in an event or class of events that is not yet being
   produced.
1. Services can be connected to create new applications
   - without modifying producer or consumer, and
   - with the ability to select a specific subset of events from a particular
     producer.

The high level mission of Knative Eventing is: **Enable asynchronous application
development through event delivery from anywhere.**

For the full mission of Knative Eventing see
[docs/mission.md](./docs/mission.md).

For complete Knative Eventing documentation, see
[Knative eventing](https://www.knative.dev/docs/eventing/) or
[Knative docs](https://www.knative.dev/docs/) to learn about Knative.

If you are interested in contributing, see [CONTRIBUTING.md](./CONTRIBUTING.md),
[DEVELOPMENT.md](./DEVELOPMENT.md) and
[Knative WORKING-GROUPS.md](https://www.knative.dev/contributing/working-groups/#eventing).

Please join
[knative-users](https://groups.google.com/forum/#!forum/knative-users) to view
planned project releases in
[roadmap](https://docs.google.com/document/d/1z0z412rL9FsBsF8kwKxG6w7sflJLKe9hIECkc7jWfOY/edit#).
