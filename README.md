# Knative Eventing

[![GoDoc](https://godoc.org/github.com/knative/eventing?status.svg)](https://godoc.org/github.com/knative/eventing)
[![Go Report Card](https://goreportcard.com/badge/knative/eventing)](https://goreportcard.com/report/knative/eventing)
[![Releases](https://img.shields.io/github/release-pre/knative/eventing.svg)](https://github.com/knative/eventing/releases)
[![LICENSE](https://img.shields.io/github/license/knative/eventing.svg)](https://github.com/knative/eventing/blob/master/LICENSE)
[![Slack Status](https://img.shields.io/badge/slack-join_chat-white.svg?logo=slack&style=social)](https://knative.slack.com)

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

Interested users should join
[knative-users](https://groups.google.com/forum/#!forum/knative-users),
