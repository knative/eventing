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

Add a field to the Trigger Filter spec allowing users to specify an expression
as a string. The expression is evaluated for each event considered by the
Trigger. If the expression evaluates to true, the event is delivered to the
Trigger's subscriber.

The evaluation environment of the expression may include:

*   Standard CloudEvents attributes like type and source
*   Custom extensions dynamically parsed from the event
*   Data fields parsed from the event, if the data content type is known to be
    parseable

Since parsing dynamic extension attributes and data fields may be costly, the
Trigger must explicitly enable it by setting boolean fields adjacent to the
expression. See [examples](#examples) for syntax.

The expression language chosen is
[CEL](https://github.com/google/cel-spec/blob/9cdb3682ba04109d2e03d9b048986bae113bf36f/doc/intro.md)
as it has the best combination of features for this purpose. See
[alternatives considered](#alternatives-considered) for comparisons to other
expression languages.

```yaml
spec:
  filter:
    cel:
      expression: >
        ce.type == "com.github.pull.create" &&
        source == "https://github.com/knative/eventing/pulls/123"
```

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

Specified with the SourceAndType field:

```yaml
spec:
  filter:
    sourceAndType:
      type: com.github.pull.create
```

Specified with a CEL expression:

```yaml
spec:
  filter:
    cel:
      expression: ce.type == "com.github.pull.create"
```

### Multiple attributes exact match

Specified with the SourceAndType field:

```yaml
spec:
  filter:
    sourceAndType:
      type: com.github.pull.create
      source: https://github.com/knative/eventing/pulls/123
```

Specified with a CEL expression:

_The `>` syntax is a standard yaml multiline string._

```yaml
spec:
  filter:
    cel:
      expression: >
        ce.type == "com.github.pull.create" &&
        ce.source == "https://github.com/knative/eventing/pulls/123"
```

### Prefix match on source

Cannot be specified with the SourceAndType field.

CEL doesn't support prefix match directly, but prefix match can be accomplished
with a regular expression:

```yaml
spec:
  filter:
    cel:
      expression: ce.source.matches("^https://github.com")
```

### Complex boolean expression

Cannot be specified with the SourceAndType field.

_The `>` syntax is a standard yaml multiline string._

```yaml
spec:
  filter:
    cel:
      expression: >
        ce.type == "com.github.pull.create" ||
        (ce.type == "com.github.issue.create" && ce.source.matches("proposals")

```

### Parsed extensions

Cannot be specified with the SourceAndType field.

_This example assumes the repository name is available as a CloudEvents extension
with name `repository`._

_The `>` syntax is a standard yaml multiline string._

```yaml
spec:
  filter:
    cel:
      parseExtensions: true
      expression: >
        ce.type == "com.github.pull.create" ||
         (ce.type == "com.github.issue.create" &&
          ext.repository == "proposals")

```

### Parsed data

Cannot be specified with the SourceAndType field.

_The `>` syntax is a standard yaml multiline string._

_Until [google/cel-go#203](https://github.com/google/cel-go/issues/203) is
resolved, dynamic integer fields in CEL must be compared as floats._

```yaml
spec:
  filter:
    cel:
      parseData: true
      expression: >
        ce.type == "dev.knative.observation" &&
        data.latency > 300.0
```

## Caveats

### Variable prefixes

Due to limitations in the CloudEvents SDK, dynamic attributes and data fields
must have a separate prefix from non-dynamic types. The examples here use
these prefixes:

| Attribute|Description                                        |
|--------|-----------------------------------------------------|
| `ce`   | Official CloudEvents attributes defined in the spec.|
| `ext`  | Extensions parsed dynamically from the event.       |
| `data` | Fields parsed dynamically from the event data.      |

This limitation may be lifted in the future, allowing official CloudEvents
attributes to use the same prefix as extensions. Extensions can then be elevated
to official attributes without changing filter expressions.

### CEL

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

## Alternatives Considered

The following alternative solutions were considered but rejected.

### Javascript or Lua as expression language

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

### Rego (OPA)

[Rego](https://www.openpolicyagent.org/docs/v0.10.7/language-reference/) is
already as a policy expression language by
[Open Policy Agent](https://www.openpolicyagent.org/). It could be an attractive
choice, but
[OPA has indicated a desire to replace Rego with CEL](https://github.com/open-policy-agent/opa/issues/811#issuecomment-401844999).
Since Rego doesn't seem to be used outside OPA, choosing Rego for Trigger
filters would likely make Triggers the only remaining use of Rego.

The possible maintenance burden of being the only Rego user makes Rego less
suitable for Trigger filtering than CEL.

### Custom expression language

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

### Structured filters

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
language runtime.

Structured filters are not as flexible as an expression language. For example,
the above Kubernetes label selectors cannot compose expressions with Boolean OR.
This lack of flexibility makes structured filters difficult to use for upstream
filter propagation.

Structured filters would need a custom implementation maintained by Knative
contributors that grows in complexity as new capabilities are added. An
off-the-shelf expression language can be embedded and immediately support
significant complexity without additional effort.

The inflexibility and implementation complexity of structured filters make this
solution less suitable for Trigger filtering than CEL. However, structured
filters can be a useful and important user interface on top of an expression
language: for example, the existing SourceAndType syntax for expressing a filter
could be implemented by generating an equivalent CEL expression. This provides
users a structured filter interface when their needs are simple, and allows them
to transition to a more complex interface when their needs require it.
