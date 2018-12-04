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

<!-- TODO(n3wscott): * [Error conditions and reporting](errors.md) -->
<!-- TODO(n3wscott): * [Sample API usage](normative_examples.md) -->

See the
[Knative Eventing Docs Architecture](https://github.com/knative/docs/blob/master/eventing/README.md#architecture)
for more high level details.
<!-- TODO(#498): Update the docs/Architecture page. -->
