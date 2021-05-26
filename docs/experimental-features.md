# Experimental features process

## Introduction

In Knative Eventing we want to keep the innovation alive, experimenting and
delivering new features without affecting the stability of the project.

This document describes the process to include new features from their
inceptions up to their stability graduation. This allows us to properly
experiment and gather user feedback, in order to stabilize new features before
supporting them.

This document describes:

- The feature stages definition
- The process from the inception to the graduation of the feature to stable
- Some technical guidelines on how to include the feature, including how to add
  new API fields and how to turn on/off experimental features.

Some technical details are left unspecified on purpose, because they tend to
depend on the features themselves.

## Scope

Experimental features might consist in:

- Behavioral/Semantic additions/changes, without affecting the APIs (CRDs). E.g.
  [#4560](https://github.com/knative/eventing/pull/4560)
- Additive changes to the APIs, adding new behaviors. E.g.
  [#5148](https://github.com/knative/eventing/issues/5148)

This process MUST be followed in all the situations above explained for changes
affecting the https://github.com/knative/eventing repository. Eventing
subprojects SHOULD follow this process too.

Breaking changes to the APIs, like modifying the API object structure, or
complete new CRDs, are _not covered_ by this document and should be introduced
using the usual API versioning, e.g. bumping from _v1_ to _v2alpha1_ or adding a
new CRD as _v1alpha1_.

This process covers both normative and non-normative features. The only
difference is that, at the end of the process, in the former case vendors are
required to implement the feature in order to remain compliant, while in the
latter case supporting the feature is optional.

## Technical overview

### Runtime feature gate

We store in a `ConfigMap` a map of enabled/disabled features. These are very
similar to
[Kubernetes Feature Gates](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/)
and
[Knative Serving feature flags](https://github.com/knative/serving/blob/master/pkg/apis/config/features.go).
When the feature is implemented, we provide a Golang API to check if the feature
is enabled or not: [`experimental.FromContext(ctx).IsEnabled(featureName)`](../pkg/apis/experimental).

The user can easily enable the features modifying a config map/environment
variable of their knative setup:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-experimental-features
  namespace: knative-eventing
data:
  my-fancy-feature: false
  another-cool-feature: true
```

### Strategies to add new APIs for experimental features

There are three main **recommended** strategies to implement APIs for new
features:

- If the feature requires a field on the API, or the feature may affect several
  parts of the API, modify the stable APIs adding the necessary fields. These
  fields **must** be optional, and it should be clearly stated in the godoc that
  the feature is experimental and may change. In the CRD yaml the field(s)
  should not be specified, and `x-kubernetes-preserve-unknown-fields` should be
  used instead. Then, in the webhook, you can reject resources with fields
  related to experimental features when validating the input CR using
  [`experimental.ValidateAPIFields()`](../pkg/apis/experimental/api_validation.go)
- Alternative to the above approach, if the feature affects a whole resource,
  and the API is a single value, that is not an object or an array of values,
  use an annotation. Then, in the webhook, you can reject resources with
  annotations related to experimental features when validating the input CR
  using
  [`experimental.ValidateAnnotations()`](../pkg/apis/experimental/api_validation.go).
  This approach is not suggested in case adding an API field is doable as
  described above, but there might be some situations when this is not possible,
  where an annotation is more suited as the feature API. For example:
  [Eventing Kafka Broker - Configuring the order of delivered events](https://github.com/knative/docs/blob/main/docs/eventing/broker/kafka-broker.md#configuring-the-order-of-delivered-events)
- If a feature is supposed to be opt-in even after its graduation to stable, for
  example an optional behavior for a specific component/set of components, make
  sure to have a flag to enable it in the place wherever is more appropriate for
  that flag to exist. E.g. a flag to enable the broker to send an additional
  header, when dispatching events, should be placed in the ConfigMap that
  configures the broker (`Broker.Spec.Config`). This makes sure the experimental
  flags are not confused with opt-in/opt-out stable features and fits into the
  already existing/implemented scoping of components configuration. The
  experimental flag to enable such a feature acts as a kill-switch, while the
  flag in the appropriate ConfigMap acts as the effective "feature interface".

It’s not required to follow such strategies, but we encourage feature designers
to stick to one of these approaches, wherever possible, in order to ensure
consistency and aligned user experience.

## Feature stages

### Stage definition

A feature can be in Alpha, Beta or GA stage. An Alpha feature means:

- Disabled by default.
- The feature is not normative at this point of the process, that is no vendor
  is required to implement it, although this is encouraged to provide feedback.
- Might be buggy. Enabling the feature may expose bugs.
- Support for the feature may be dropped at any time without notice.
- The API may change in incompatible ways in a later software release without
  notice.
- Recommended for use only in short-lived testing clusters, due to increased
  risk of bugs and lack of long-term support.

A Beta feature means:

- Enabled by default.
- The feature is not normative at this point of the process, that is no vendor
  is required to implement it, although this is encouraged to provide feedback.
- The feature is well tested. Enabling the feature is considered safe.
- Support for the overall feature will not be dropped, though details may
  change.
- The schema and/or semantics of objects may change in incompatible ways in a
  subsequent beta or stable release. When this happens, we will provide
  instructions for migrating to the next version. This may require deleting,
  editing, and re-creating API objects. The editing process may require some
  thought. This may require downtime for applications that rely on the feature.
- Recommended for only non-business-critical uses because of potential for
  incompatible changes in subsequent releases. If you have multiple clusters
  that can be upgraded independently, you may be able to relax this restriction.

A General Availability (GA) feature is also referred to as a stable feature. It
means:

- The feature is always enabled; you cannot disable it.
- The corresponding experimental feature gate is no longer needed.
- New API is properly documented in the
  [knative/specs](https://github.com/knative/specs/) repository, and it must be
  specified if it's normative or not.
- Stable versions of features will appear in released software for many
  subsequent versions, following the requirements described in the
  [Stable maturity level](https://github.com/knative/community/blob/main/mechanics/MATURITY-LEVELS.md#stable).

These features stages are heavily influenced by
[Kubernetes feature stages](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/#feature-stages).

### Graduation constraints

These hard constraints MUST be satisfied when planning/executing the graduation
of the features through the several phases, although depending on the
discussions, the actual timeline may end up being longer:

- For a feature to graduate from _Alpha_ to _Beta_, there MUST be at least 1
  release span from its introduction to the _Beta_ graduation. E.g. if the
  feature was introduced in 0.15, it can be graduated to _Beta_ in 0.16
- For a feature to graduate from _Beta_ to _GA_, there MUST be at least 2
  releases span from its _Beta_ graduation to the _GA_ graduation. E.g. if the
  feature became _Beta_ in 0.15, it can be graduated to _GA_ in 0.17
- A feature MUST NOT jump from _Alpha_ to _GA_ without going through _Beta_.

### Quality requirements

- **Alpha**
  - Documentation of the feature flag and respective usage
  - Good unit test coverage
  - End to end testing covering _at least_ the usage described in the user
    documentation
- **Beta**
  - More than 80% of unit test coverage
  - Extensive end to end testing
- **Stable**
  - Extensive user documentation
  - Extensive code documentation
  - Conformance tests (if needed)
  - If the feature adds new fields in the CRDs, the CRD schema must be properly
    documented, and fields should be documented in the spec

## Feature proposal, development and graduation process

Any member of the community can propose a new feature through the below
described process.

I refer to _Feature owner_ as the individual/group of people that wants to
include the feature.

In short, this process is composed of a phase of discussion for each feature
stage, followed by a consensus period through lazy consensus. We suggest the
feature owner move to the consensus period only when there is **enough
confidence** in the discussions, avoiding incurring in vetos from any member of
the community or late disagreement.

In detail:

1. Feature owner develops a feature track document and a Github issue as an
   “umbrella issue”, or alternatively just a Github Issue if the feature is
   _small enough_. The issue and/or the document is necessary to bootstrap the
   discussion. The document/Github issue MUST be explicit about:
   1. The expected roadmap for the feature, including on which release the
      graduation phases are expected. This is just a vague indication to the
      community of the timeline for the feature from its inception to the
      stability, and it’s not meant to be followed rigorously throughout the
      process. This may also include some expected criteria, in addition to the
      quality requirements described below. For example, let’s assume the
      feature owner is proposing to add resource fields conforming to a new spec
      from CNCF (e.g. like
      [#5204](https://github.com/knative/eventing/issues/5204)), a reasonable
      roadmap could be: _Inception is expected in 0.23, graduation to beta in
      0.25 to give time to vendors to implement the new fields, graduation to GA
      as soon as the referred CNCF spec is considered stable_
   1. Which WGs are affected and should watch out for the feature graduation
      process. They can be more than one.
1. Spread the Github issue throughout all Knative channels: Slack, WG meetings,
   mails. Keep the feature discussion open, possibly inside the Github Issue
   and/or in the knative-dev mailing list and/or in Github Discussions,
   depending on your preference. Avoid synchronous discussions as much as
   possible, because these can’t be followed by everybody. During this phase,
   the feature owner needs to make sure the community agrees with the inclusion
   of the feature and, on the other hand, community members should speak up to
   express strong disagreement, providing proper reasons about it.
1. Feature owner implements the first version of the feature (in one or more
   PRs), following the proposed implementation strategies whenever is possible
   and mark it as experimental. The feature should have the above described
   quality requirements.
1. After the first round of the discussion is done, and the feature
   implementation is ready to be merged. In order to reach consensus on
   proceeding with the inclusion, feature owner MUST use the following lazy
   consensus process:
   1. Send a mail on a new thread to the knative-dev mailing list, or
      alternatively open a new issue in the eventing repository, to inform about
      the imminent inclusion of the feature. Be explicit about **the time span**
      for people to reply (at least 1 week). Include significant discussions/PRs
      that have been made. In order to reach as many people as possible, share
      on Slack and during WG meetings the thread/issue link.
   1. If anybody expresses strong disagreement (no matter if contributor, user,
      approver, lead) **inside the mail thread/Github issue used to agree**, the
      feature PR(s) can’t be merged, and the community should go back to the
      previous stage, otherwise the feature can be merged by anybody and be
      released as _Alpha_. This strong disagreement must be followed by precise
      reasons and a suggested course of action.
1. Feature owner should collect user feedback and keep discussing openly with
   the community improvements to the feature. Try to keep the “broader”
   discussions about the feature itself inside the initial thread/channel where
   the discussion started.
1. After the second round of discussions is done and the quality requirements
   for Beta are met, the feature is ready for the _Beta_ graduation. Feature
   owner should prepare the PR to enable the feature by default, which can be
   merged after reaching consensus with the following lazy consensus process:
   1. Send a mail on a new thread to the knative-dev mailing list, or
      alternatively open a new issue in the eventing repository, to inform about
      the imminent graduation of the feature. Be explicit about **the time
      span** for people to reply (at least 1 week). Include significant
      discussions/PRs that have been made. In order to reach as many people as
      possible, share on Slack and during WG meetings the thread/issue link.
   1. If any approver or lead expresses strong disagreement **inside the mail
      thread/Github issue used to agree**, the feature PR can’t be merged, and
      the community should go back to the discussions, otherwise the feature can
      be merged by anybody and be released as _Beta_. This strong disagreement
      must be followed by precise reasons and a suggested course of action.
   1. **In extreme situations**, leads together can veto at most once on the
      graduation, providing a specific course of action, like delaying the
      graduation or removing the feature. We strongly encourage feature owner to
      never reach such a situation.
1. Feature owner should collect user feedback and keep discussing openly with
   the community improvements to the feature. As in the previous steps, try to
   keep the “broader” discussions about the feature itself inside the initial
   thread/channel where the discussion started.
1. Breaking the APIs is still allowed, but discouraged at this point. If the
   experimental API is annotation based, consider moving it inside the resources
   spec. During the _Beta_ lifecycle, we suggest to provide migration scripts
   for breaking changes, if possible, although this is not mandatory.
1. After the third round of discussions is done and the quality requirements for
   Stable are met, the feature is ready for the _Stable_ graduation. Feature
   owner should prepare the PR to remove the feature flag, the PR to include in
   the specs repo the new API and, if necessary, the PR to move the annotation
   flag to the resource spec. These PRs can be merged after reaching consensus
   with the following lazy consensus process:
   1. Send a mail on a new thread to the knative-dev mailing list, or
      alternatively open a new issue in the eventing repository, to inform about
      the imminent graduation of the feature. Be explicit about **the time
      span** for people to reply (at least 10 workdays). Include significant
      discussions/PRs that have been made. In order to reach as many people as
      possible, share on Slack and during WG meetings the thread/issue link.
   1. If any approver or lead expresses strong disagreement **inside the mail
      thread/Github issue used to agree**, the feature PR can’t be merged, and
      the community should go back to the discussions, otherwise the PRs can be
      merged by anybody and be released as _Stable_. This strong disagreement
      must be followed by precise reasons and a suggested course of action.
   1. **In extreme situations**, all leads together can veto at most once on the
      graduation, providing a specific course of action, like delaying the
      graduation or removing the feature. We strongly encourage feature owner to
      never reach such a situation.
