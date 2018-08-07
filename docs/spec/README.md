# Knative Eventing API spec

This directory contains the specification of the Knative Eventing API, which is
implemented in [`channels.knative.dev`](/pkg/apis/channels/v1alpha1),
[`feeds.knative.dev`](/pkg/apis/feeds/v1alpha1) and
[`flows.knative.dev`](/pkg/apis/flows/v1alpha1) and verified via [the e2e
test](/test/e2e).

**Updates to this spec should include a corresponding change to the API
implementation for [channels](/pkg/apis/channels/v1alpha1),
[feeds](/pkg/apis/feeds/v1alpha1) or [flows](/pkg/apis/feeds/v1alpha1) and [the
e2e test](/test/e2e).**

Docs in this directory:

* [Motivation and goals](motivation.md)
* [Resource type overview](overview.md)
<!-- TODO(n3wscott): * [Knative Eventing API spec](spec.md) -->
<!-- TODO(n3wscott): * [Error conditions and reporting](errors.md) -->
<!-- TODO(n3wscott): * [Sample API usage](normative_examples.md) -->

<!-- TODO(evankanderson):
`This may be the right place, but it seems like it would be useful to include a
section on either "Known issues" with the API and/or patterns that we've agreed
upon. In particular, the use of containers/images for extension of Buses and
Sources is probably worth highlighting somewhere.`

The Parameters/ParametersFrom pattern for passing args to the images is
probably also worth documenting.
-->