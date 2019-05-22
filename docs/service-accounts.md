# Kubernetes Service Accounts

Knative Eventing has many pieces. Many of those pieces run Kubernetes Pods,
meaning they run as Kubernetes ServiceAccounts. This document describes
Knative Eventing's recommended use and re-use of ServiceAccounts.

## General Philosophy

Knative Eventing's general philosophy is that it is easier to merge
ServiceAccounts than to split them. So if undecided about what to do, err on
the side of finer grained permissions.
## Trade offs

### Finer Grained Permissions

**Pros**:
- Mistakes are more bounded in the amount of damage they can do.
- Easier to track which component caused a particular change.

**Cons**:
- More complex to manage. There are more entities in the system and each one must be kept up to date.
- When debugging, there is much more that you will need to be aware of.

## Core

Core pieces of Knative Eventing MUST use distinct ServiceAccounts for every
binary and MUST give each of those ServiceAccounts minimal RBAC permissions.

When multiple logical pieces are combined into a single binary, they have only
one ServiceAccount. E.g. the Eventing Controller has multiple logical
controllers inside it. It could be split into separate binaries, but that would
drastically increase the resource requirements to run Knative Eventing.

The reasoning behind fine grained permissions is that the core is required for
all users of Knative Eventing. Some users will want fine grained permissions. In
addition, the Knative team can do most of the work necessary for these
permissions so that users don't need to think about them.

The following pieces are all considered part of the 'core':
- Eventing Controller
    - Broker Controller
    - Default Channel Controller
    - Subscription Controller
    - Trigger Controller
- Eventing Webhook
- Broker `Deployments` (both Ingress and Filter)

## Channels

Distinct Channel types SHOULD use different ServiceAccounts. Further, each Channel type SHOULD
use different ServiceAccounts for their control plane and data plane Pods.

All existing Channel implementations use two binaries, controller and
dispatcher. They all have distinct ServiceAccounts for each, as they require
distinct permissions. In the future, this may be broken down even further by
splitting the dispatcher into receiving and emitting binaries. If they require
distinct permissions, then they SHOULD use different ServiceAccounts, each
scoped to the exact set of permissions required.

## Sources

Sources are different, because unlike [Core](#core) and
[Channels](#channels) event consumers are expected to both create and manage many
different Sources. So the cost of distinct ServiceAccounts is more pronounced.

Most Event Source CRDs have two components: control plane and data plane.
control plane ServiceAccounts SHOULD be unique to that Event Source CRD. The
control plane and data plane SHOULD NOT use the same ServiceAccount. If the
data plane needs no Kubernetes permissions, then it MAY use the `default`
ServiceAccount.

Most Event Source's data plane component do not need any Kubernetes level
permission, and thus MAY use `default`.

Event Sources MUST NOT require RBAC permissions be granted to the `default`
ServiceAccount. If a permission is required, then the Event Source MUST use a
ServiceAccount other than `default` or allow the user to specify the
ServiceAccount to be used.
