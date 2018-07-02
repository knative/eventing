# Knative Eventing API spec

This directory contains the specification of the Knative Eventing API, which is
implemented in [`channels.knative.dev`](/pkg/apis/channels/v1alpha1) and
[`feeds.knative.dev`](/pkg/apis/feeds/v1alpha1) and verified via [the e2e
test](/test/e2e).

**Updates to this spec should include a corresponding change to the
API implementation for [channels](/pkg/apis/channels/v1alpha1) or
in [feeds](/pkg/apis/feeds/v1alpha1) and [the e2e test](/test/e2e).**

Docs in this directory:

* [Motivation and goals](motivation.md)
* [Resource type overview](overview.md)
* [Knative Eventing API spec](spec.md)
* [Error conditions and reporting](errors.md)
* [Sample API usage](normative_examples.md)