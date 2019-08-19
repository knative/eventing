# Development

This doc explains how to setup a development environment so you can get started
[contributing](./CONTRIBUTING.md) to Knative Eventing. Also take a look at
[the development workflow](./CONTRIBUTING.md#workflow) and
[the test docs](./test/README.md).

## Getting started

1. Setup [Knative Serving](http://github.com/knative/serving)
1. [Create and checkout a repo fork](#checkout-your-fork)
1. [Install a channel implementation](#install-channels)

Once you meet these requirements, you can
[start the eventing-controller](#starting-eventing-controller).

Before submitting a PR, see also [CONTRIBUTING.md](./CONTRIBUTING.md).

### Requirements

You must have the core of [Knative](http://github.com/knative/serving) running
on your cluster.

You must have [`ko`](https://github.com/google/ko) installed.

### Checkout your fork

The Go tools require that you clone the repository to the
`src/knative.dev/eventing` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [fork of this repo](https://help.github.com/articles/fork-a-repo/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/github.com/knative
cd ${GOPATH}/src/github.com/knative
git clone git@github.com:${YOUR_GITHUB_USERNAME}/eventing.git
cd eventing
git remote add upstream git@github.com:knative/eventing.git
git remote set-url --push upstream no_push
```

_Adding the `upstream` remote sets you up nicely for regularly
[syncing your fork](https://help.github.com/articles/syncing-a-fork/)._

Once you reach this point you are ready to do a full build and deploy as
follows.

## Install Channels

Install the
[In-Memory-Channel](https://github.com/knative/eventing/tree/master/config/channels/in-memory-channel)
since this is the
[default channel](https://github.com/knative/docs/blob/master/docs/eventing/channels/default-channels.md).

```shell
ko apply -f config/channels/in-memory-channel/
```

Depending on your needs you might want to install other
[channel implementations](https://github.com/knative/docs/blob/master/docs/eventing/channels/channels-crds.md).

## Starting Eventing Controller

Once you've [setup your development environment](#getting-started), stand up
`Knative Eventing` with:

```shell
ko apply -f config/
```

You can see things running with:

```shell
$ kubectl -n knative-eventing get pods
NAME                                   READY     STATUS    RESTARTS   AGE
eventing-controller-59f7969778-4dt7l   1/1       Running   0          2h
```

You can access the Eventing Controller's logs with:

```shell
kubectl -n knative-eventing logs $(kubectl -n knative-eventing get pods -l app=eventing-controller -o name)
```

## Iterating

As you make changes to the code-base, there are two special cases to be aware
of:

- **If you change a type definition ([pkg/apis/](./pkg/apis/.)),** then you must
  run [`./hack/update-codegen.sh`](./hack/update-codegen.sh).
- **If you change a package's deps** (including adding external dep), then you
  must run [`./hack/update-deps.sh`](./hack/update-deps.sh).

These are both idempotent, and we expect that running these at `HEAD` to have no
diffs.

Once the codegen and dependency information is correct, redeploying the
controller is simply:

```shell
ko apply -f config/controller.yaml
```

Or you can [clean it up completely](#clean-up) and start again.

## Tests

Running tests as you make changes to the code-base is pretty simple. See
[the test docs](./test/README.md).

## Clean up

You can delete `Knative Eventing` with:

```shell
ko delete -f config/
```

## Telemetry

To access Telemetry see:

- [Accessing Metrics](https://www.knative.dev/docs/serving/accessing-metrics/)
- [Accessing Logs](https://www.knative.dev/docs/serving/accessing-logs/)
- [Accessing Traces](https://www.knative.dev/docs/serving/accessing-traces/)
