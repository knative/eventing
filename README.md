# Knative Eventing

[![GoDoc](https://godoc.org/github.com/knative/eventing?status.svg)](https://godoc.org/github.com/knative/eventing)
[![Go Report Card](https://goreportcard.com/badge/knative/eventing)](https://goreportcard.com/report/knative/eventing)

This repository contains a work-in-progress eventing system that is designed to
address a common need for cloud native development:

1. Services are loosely coupled during development and deployed independently
2. A producer can generate events before a consumer is listening, and a consumer
   can express an interest in an event or class of events that is not yet being
   produced.
3. Services can be connected to create new applications
   - without modifying producer or consumer, and
   - with the ability to select a specific subset of events from a particular
     producer.

For complete Knative Eventing documentation, see
[Knative eventing](https://github.com/knative/docs/tree/master/eventing) or
[Knative docs](https://github.com/knative/docs/) to learn about Knative.

If you are interested in contributing, see [CONTRIBUTING.md](./CONTRIBUTING.md),
[DEVELOPMENT.md](./DEVELOPMENT.md) and
[Knative WORKING-GROUPS.md](https://github.com/knative/docs/blob/master/community/WORKING-GROUPS.md#events).
