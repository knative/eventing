# Advanced Filtering for Triggers Proposal

## Problem

As an `Event Consumer` I want to filter events in triggers with a variety of
strategies beyond exact match on predefined attributes. Filtering strategies
that have been requested include:

*   Filtering by source or type prefix or regular expression
*   Filtering by a custom extension attribute
*   Filtering by data fields

The existing filter capability of exact match on source and type is simple to
understand and use, but doesn't support these more advanced filtering scenarios.

## Objective

Design a flexible filtering mechanism that supports all of the advanced use
cases identified and can also implement simpler cases like exact match.

The filtering mechanism MUST be safe and efficient. It SHOULD be compatible with
a future
[upstream propagation solution](https://github.com/knative/eventing/issues/934)
for delivery efficiency at scale.

Filters should be easy for the user to specify and simple to interpret at a
glance.

## Requirements

*   Support all of the advanced filtering use cases identified.
*   Can implement simpler cases like exact match.
*   Safe and secure when embedded in multi-tenant processes.
*   Compatible with upstream filter propagation.
*   Easily specified by the user.
*   Simple for users to interpret.

## Proposed Solution

Extend the Trigger's filter syntax to allow specifying an expression written in
[CEL](https://github.com/google/cel-spec). The existing `sourceAndType` filter
will be deprecated and replaced with an `attributes` map that allows filtering
on any CloudEvents attribute.

### Add CEL expression filter

Add an `expression` filter to the Trigger allowing users to specify an
expression as a string. The expression is evaluated for each event considered
by the Trigger. If the expression evaluates to true, the event is delivered to
the Trigger's subscriber.

The evaluation environment of the expression may include:

*   Official CloudEvents attributes like type and source.
*   Custom extension attributes dynamically parsed from the event.
*   Data fields parsed from the event, if the data content type is known to be
    parseable

The expression language chosen is
[CEL](https://github.com/google/cel-spec/blob/9cdb3682ba04109d2e03d9b048986bae113bf36f/doc/intro.md)
as it has the best combination of features for this purpose. See
[alternatives considered](#alternatives-considered) for comparisons to other
expression languages.

```yaml
spec:
  filter:
    expression: >
      ce.type == "com.github.pull.create" &&
      ce.source == "https://github.com/knative/eventing/pulls/123"
```

### Validate expression via webhook

Add a check to the Trigger webhook that validates the expression's syntax and
rejects the request if an error is encountered. This will not catch errors
arising from use of dynamic extension attributes or data fields - these can only be
caught at evaluation time.

### Add attributes map filter and deprecate SourceAndType

Add an `attributes` filter that allows the user to specify equality for any
CloudEvents context attribute. Multiple attributes may be specified. The event
is delivered if all specified attribute values match the event exactly (Boolean
`AND`).

Extensions may be specified as attributes. Nested extension fields must be
specified using a dot syntax, e.g. `field1.field2.field3`.

Deprecate the SourceAndType filter as its functionality is subsumed by the
`attributes` filter.

### Transform all filters into CEL expressions

Implement the `attributes` filter (and reimplement the deprecated SourceAndType
filter) as a transformation to an equivalent CEL expression. This makes CEL the
only code path that does filtering, both simplifying maintenance and making the
implementation of filtering easier in other components (such as upstream event
sources).

### Document policy for filter specifications

To strike a balance between flexibility, maintainability, and user convenience,
adopt the following principles:

* Only one top-level filter may be used per Trigger. A Trigger that specifies
  more than one is rejected.
* All filters must be transformable to an equivalent CEL expression.
* Additional filter specifications may be added if indicated by user feedback.


## Implementation details

### Variables in the expression

| Prefix |Description |
|--------|------------|
| `ce`   | Official CloudEvents attributes defined in the spec, plus extension attributes if requested. |
| `data` | Fields parsed from the event data, if requested. |

Official attributes and extension attributes are both available to the
expression under the prefix `ce`. This co-mingling is intended to ease the
promotion of extension attributes to official attributes. The prefix `ce` was
chosen for succintness and its suggestion that CloudEvents is the operative
specification.

Data fields are available to the expression under the prefix `data`, if the data
serialization format used is known and can be parsed by the filtering
implementation.

### Why CEL?

CEL is the
[Common Expression Language](https://github.com/google/cel-spec/blob/9cdb3682ba04109d2e03d9b048986bae113bf36f/doc/langdef.md).
It is designed for expressing security policy and protocols, and has the
following
[guiding philosophy](https://github.com/google/cel-spec/blob/9cdb3682ba04109d2e03d9b048986bae113bf36f/README.md):

*   **Small and fast.** CEL is not Turing complete, so it is easy to understand
    and implement.
*   **Extensible.** CEL is designed to be embedded and can be extended with new
    functionality.
*   **Developer-friendly.** The language spec was based on experience and
    usability testing from implementing Firebase Rules.

Additional properties of CEL that make it attractive for Trigger Filtering
include:

*   **Familiar expression syntax.** Filter expressions written in CEL look and
    behave like conditional expressions written in the most popular languages.
*   **Expressions can be compiled and cached for efficiency.** We expect users
    will create many filters that change rarely in comparison to frequency of
    evaluations. This usage pattern works in favor of an expression language
    that can be cached in compiled form.
*   **Expressions are serializable and composable.** These properties are useful
    in implementing upstream filter propagation. For example, a Broker could
    advertise a single CEL expression composed of the union of its Triggers'
    filters, allowing upstream sources to filter events before publishing to the
    Broker.

CEL is
[memory-safe, side-effect-free, and terminating](https://github.com/google/cel-spec/blob/master/doc/langdef.md#overview),
making it safe and secure to embed in a multi-tenant dispatcher process.

## Examples

### Single attribute exact match

Specified with the SourceAndType filter:

```yaml
spec:
  filter:
    sourceAndType:
      type: com.github.pull.create
```

Specified with the attributes filter:

```yaml
spec:
  filter:
    attributes:
      type: com.github.pull.create
```

Specified with the expression filter:

```yaml
spec:
  filter:
    expression: ce.type == "com.github.pull.create"
```

### Multiple attributes exact match

Specified with the SourceAndType filter:

```yaml
spec:
  filter:
    sourceAndType:
      type: com.github.pull.create
      source: https://github.com/knative/eventing/pulls/123
```

Specified with the attributes filter:

```yaml
spec:
  filter:
    attributes:
      type: com.github.pull.create
      source: https://github.com/knative/eventing/pulls/123
```

Specified with the expression filter:

_The `>` syntax is a standard yaml multiline string._

```yaml
spec:
  filter:
    expression: >
      ce.type == "com.github.pull.create" &&
      ce.source == "https://github.com/knative/eventing/pulls/123"
```

### Prefix match on source

Cannot be specified with the SourceAndType filter or the attributes filter.

CEL supports multiple options for prefix match. The simplest is `startsWith`:

```yaml
spec:
  filter:
    expression: ce.source.startsWith("https://github.com")
```

CEL also implements shell-style wildcards with `match`:

```yaml
spec:
  filter:
    expression: ce.source.match("https://github.com*")
```

For the most complex matching needs, CEL implements
[RE2-style](https://github.com/google/re2/wiki/Syntax) regular expressions with
`matches`:

```yaml
spec:
  filter:
    expression: ce.source.matches("https://github.com.*")
```

### Complex boolean expression

Cannot be specified with the SourceAndType filter.

_The `>` syntax is a standard yaml multiline string._

```yaml
spec:
  filter:
    expression: >
      ce.type == "com.github.pull.create" ||
      (ce.type == "com.github.issue.create" && ce.source.matches("proposals")
```

### Exact match on official and extension attribute

Cannot be specified with the SourceAndType filter.

_This example assumes the repository name is available as a CloudEvents
extension `{"repository":proposals"}`._

Specified with the attributes filter:

```yaml
spec:
  filter:
    attributes:
      type: com.github.issue.create
      repository: proposals
```

_The `>` syntax is a standard yaml multiline string._

```yaml
spec:
  filter:
    expression: >
      ce.type == "com.github.issue.create" &&
      ce.repository == "proposals"
```

### Exact match on extension with dotted name

Cannot be specified with the SourceAndType filter.

_This example assumes the repository name is available as a CloudEvents
extension `{"github.repository":proposals"}`._

Specified with the attributes filter:

```yaml
spec:
  filter:
    attributes:
      github.repository: proposals
```

_The `>` syntax is a standard yaml multiline string._

```yaml
spec:
  filter:
    expression: >
      ce["github.repository"] == "proposals"
```

### Exact match on nested extension

Cannot be specified with the SourceAndType filter or the attributes filter.

_This example assumes the repository name is available as a **nested**
CloudEvents extension `{"github":{"repository":"proposals"}}`._

_The `>` syntax is a standard yaml multiline string._

```yaml
spec:
  filter:
    expression: >
      ce.github.repository == "proposals"
```

### Exact match on data string field

Cannot be specified with the SourceAndType filter or the attributes filter.

_This example assumes the event data is the parseable equivalent of
`{"user":{"id":"abc123"}`._

```yaml
spec:
  filter:
    expression: >
      user.id == "abc123"
```

### Inequality on data number field

Cannot be specified with the SourceAndType filter or the attributes filter.

_This example assumes the event data is the parseable equivalent of
`{"latency":300}`._

_The `>` syntax is a standard yaml multiline string._

_Until [google/cel-go#203](https://github.com/google/cel-go/issues/203) is
resolved, dynamic integer fields in CEL must be compared as floats._

```yaml
spec:
  filter:
    expression: >
      ce.type == "dev.knative.observation" &&
      data.latency > 300.0
```

## Caveats

### Choice of CEL

The main caveat is the choice of CEL as expression language.

CEL is new and the language is not officially supported by Google. Participation
by non-Google contributors is low. OPA is planning to replace their policy
language Rego with CEL, but
[no timeline has been announced for this migration](https://github.com/open-policy-agent/opa/issues/811#issuecomment-401844999).
The KrakenD API Gateway has announced support for CEL
[here](https://medium.com/devops-faith/krakend-api-gateway-0-9-released-9427c249dbcd).

The CEL language is new and has limited exposure, despite being informed by
experience and research at Google. We may discover issues that make it less
suitable for Trigger filtering (e.g.
[google/cel-go#203](https://github.com/google/cel-go/issues/203).

If CEL turns out to be a liability, a new expression language could be added
alongside CEL without breaking existing triggers, allowing for an orderly
deprecation.

For example, a javascript filter could be added without disrupting existing CEL
filters:

```yaml
spec:
  filter:
    javascript:
      expression: ce.type == "com.github.pull.create"
```

### Expression language versioning

Embedding an expression language into the Trigger specification means that we
effectively have two versions exposed to the user: the Trigger CRD APIVersion and
the expression language version. As the CEL spec evolves, backward incompatible
changes may be made that would require a version change in the Trigger.

I propose that we rely on the Trigger CRD APIVersion to encode both the Trigger
version and the CEL version. This ensures that Triggers with the same APIVersion
always have the same behavior, and the upgrade semantics are the same as those
for other Kubernetes objects.

There is a concern that tying the CEL version to the Trigger APIVersion could
block users from upgrading, because they need to upgrade all of their
expressions at the same time as a Knative upgrade. If no automatic upgrade is
possible, we should support both CEL versions for one or more Knative releases.
It might be necessary to add a Trigger field specifying the version of CEL that
should evaluate the expression.

### Numeric and nested extension values

Some CloudEvents transports (such as the HTTP binary content mode) may be unable
to differentiate numeric and string values for top-level or nested extension
attributes. This is still being discussed by the CloudEvents working group (see
https://github.com/cloudevents/spec/pull/413).

Until this is resolved, the type of numeric values parsed in CloudEvents
extensions may be unpredictable.

### Single top-level filter

Top-level filters are now explicitly exclusive: only one may be specified. This
is done to ensure that Trigger filters are easily understood by the user.
Composable filters at the top-level would have unclear composition logic: are
they composed by Boolean **OR** or **AND**? The user must be aware of the
implicit semantics to understand the result of the trigger. 

If there is user demand for structured filter composition, top-level `or` and
`and` filters can be introduced that indicate how filter elements are composed.
This example is presented as an illustration only; it is not an element of the
proposal.

```yaml
spec:
  filter:
    and:
    - attributes:
        type: com.github.pull.create
    - expression: ce.source.match("knative")
```

## Alternatives Considered

The following alternative solutions were considered but rejected.

### Alternate expression language

#### Javascript or Lua

Trigger filtering is a very limited case for an embedded language. All that is
needed from a language is the ability to write conditions that evaluate to true
or false.

Javascript and Lua are commonly used as embedded languages, but their
capabilities extend well beyond the needs of Trigger filtering. Those
capabilities come with complexity: Javascript and Lua programs may not provably
terminate, may have memory leaks, and may consume excessive amounts of CPU. All
these cases would need to be guarded against by any process involved in
filtering. If the expression language is used for upstream filter propagation,
this could be a large number of diverse processes.

Due to this additional implementation complexity, both Javascript and Lua have
been judged less suitable for Trigger filtering than CEL.

#### Rego

[Rego](https://www.openpolicyagent.org/docs/v0.10.7/language-reference/) is
already as a policy expression language by
[Open Policy Agent](https://www.openpolicyagent.org/). It could be an attractive
choice, but
[OPA has indicated a desire to replace Rego with CEL](https://github.com/open-policy-agent/opa/issues/811#issuecomment-401844999).
Since Rego doesn't seem to be used outside OPA, choosing Rego for Trigger
filters would likely make Triggers the only remaining use of Rego.

The possible maintenance burden of being the only Rego user makes Rego less
suitable for Trigger filtering than CEL.

#### Custom expression language

We could develop our own custom expression language for Trigger filters. This
might be attractive since we could design the language for the needs of Trigger
filters specifically.

A possible example is Kubernetes' set-based label selectors:

```
environment in (production, qa)
tier notin (frontend, backend)
partition
!partition
```

The disadvantage of this approach is that the Knative Eventing contributors
would be responsible for implementing, documenting, and maintaining the
language. CEL already meets the current needs of Trigger filters and is
extensible to support future needs.

The implementation and maintenance work required to develop a custom expression
language make this solution less suitable for Trigger filtering than CEL.

### Use structured filters with no expression language

Instead of using an expression language, we could design a system of structured
filters expressible as yaml or JSON, similar to Kubernetes set-based label
selectors:

```yaml
matchExpressions:
  - {key: tier, operator: In, values: [cache]}
  - {key: environment, operator: NotIn, values: [dev]}
```

One advantage of structured filters is that their schema can be expressed in an
OpenAPI document and syntax-checked at creation time without embedding a
language runtime. Another is that simple structured filters may be easier to
implement in many languages than a CEL runtime.

Structured filters are not as flexible as an expression language. For example,
the above Kubernetes label selectors cannot compose expressions with Boolean OR.
This lack of flexibility makes structured filters difficult to use for upstream
filter propagation.

Structured filters would need a custom implementation maintained by Knative
contributors that grows in complexity as new capabilities are added. An
off-the-shelf expression language can be embedded and immediately support
significant complexity without additional effort.

The inflexibility and implementation complexity of structured filters make this
solution less suitable for Trigger filtering than CEL.

However, structured filters are a useful and important user interface _in
addition to_ an expression language. Structured filters like the `attributes`
filter or `SourceAndType` filter  should be implemented by transformation to an
equivalent CEL expression. This provides users a structured filter interface
for simple needs while also preserving the benefits of a single well-specified
mechanism for evaluating filters.

### Alternate filter specification

#### SourceAndType

```yaml
spec:
  filter:
    sourceAndType:
      source: https://github.com
      type: com.github.pull
```

Only supports source and type fields, not subject or other fields.

#### Extra level for expression options

```yaml
spec:
  filter:
    cel:
      expression: ce.subject == "knative/eventing/pull/23"
```

The ability to specify expression options was not sufficiently useful to justify
the extra level of nesting, plus we don't have a current need for expression
options.

If expression options are needed later, we can add this syntax back.

#### Multiple top-level filters

```yaml
spec:
  filter:
    attributes:
      source: https://github.com
      type: com.github.pull
    expression: ce.subject == "knative/eventing/pull/23"
```

The result of this filter is unclear: does the filter pass if all filters pass
or any filters pass?

If we later want composable filters, we can add top-level `or` and `and`
filters.

#### matchSource and matchTypes

```yaml
spec:
  filter:
    matchTypes: com.github*
    matchSource: https://github.com*
    expression: ce.subject == "knative/eventing/pull/23"
```

Only supports matching on type and source, not subject or other fields. The
result of the filter is unclear due to implicit composition, same as
[Multiple top-level filters](#multiple-top-level-filters).

## Open Questions

### Evaluation error handling

Should the filter fail open (accept) or fail closed (reject)? How should evaluation errors be surfaced to the user?