# Knative Eventing API spec

This directory contains the specification of the Knative Eventing API, which is
implemented in [`eventing.knative.dev`](/pkg/apis/eventing/v1alpha1) and
verified via [the e2e test](/test/e2e).

**Updates to this spec should include a corresponding change to the API
implementation for [eventing](/pkg/apis/eventing/v1alpha1) and
[the e2e test](/test/e2e).**

Docs in this directory:

- [Motivation and goals](motivation.md)
- [Resource type overview](overview.md)
- [Interface contracts](interfaces.md)
- [Object model specification](spec.md)
- [Channel specification](channel.md)
- [Sources specification](sources.md)
- [Error conditions and reporting](https://knative.dev/docs/serving/spec/knative-api-specification-1.0/#error-signalling)

See the
[Knative Eventing Docs Architecture](https://www.knative.dev/docs/eventing/#architecture)
for more high level details.

<!-- TODO(#498): Update the docs/Architecture page. -->
