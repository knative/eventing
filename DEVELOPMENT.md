# Development

This doc explains how to setup a development environment so you can get started
[contributing](./CONTRIBUTING.md) to Elafros Bindings. Also take a look at [the
development workflow](./CONTRIBUTING.md#workflow) and [the test docs](./test/README.md).

## Getting started

1. Setup [Elafros](http://github.com/elafros/elafros)
1. [Create and checkout a repo fork](#checkout-your-fork)

Once you meet these requirements, you can [start Binding](#starting-binding)!

Before submitting a PR, see also [CONTRIBUTING.md](./CONTRIBUTING.md).

### Requirements

You must have the core of [Elafros](http://github.com/elafros/elafros) running on your cluster:

### Checkout your fork

The Go tools require that you clone the repository to the `src/github.com/elafros/eventing` directory
in your [`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own [fork of this repo](https://help.github.com/articles/fork-a-repo/)
2. Clone it to your machine:
  ```shell
  mkdir -p ${GOPATH}/src/github.com/elafros
  cd ${GOPATH}/src/github.com/elafros
  git clone git@github.com:${YOUR_GITHUB_USERNAME}/eventing.git
  cd eventing
  git remote add upstream git@github.com:elafros/eventing.git
  git remote set-url --push upstream no_push
  ```

_Adding the `upstream` remote sets you up nicely for regularly [syncing your
fork](https://help.github.com/articles/syncing-a-fork/)._

Once you reach this point you are ready to do a full build and deploy as described [here](./README.md#start-elafros).

## Starting Binding

Once you've [setup your development environment](#getting-started), stand up `Elafros Binding` with:

```shell
ko apply -f config/
```

You can see things running with:
```shell
$ kubectl -n bind-system get pods
NAME                               READY     STATUS    RESTARTS   AGE
bind-controller-59f7969778-4dt7l   1/1       Running   0          2h
```

You can access the Binding Controller's logs with:

```shell
kubectl -n bind-system logs $(kubectl -n bind-system get pods -l app=bind-controller -o name)
```

## Iterating

As you make changes to the code-base, there are two special cases to be aware of:
* **If you change a type definition ([pkg/apis/bind/v1alpha1/](./pkg/apis/bind/v1alpha1/.)),** then you must run [`./hack/update-codegen.sh`](./hack/update-codegen.sh).
* **If you change a package's deps** (including adding external dep), then you must run
  [`./hack/update-deps.sh`](./hack/update-deps.sh).

These are both idempotent, and we expect that running these at `HEAD` to have no diffs.

Once the codegen and dependency information is correct, redeploying the controller is simply:
```shell
ko apply -f config/controller.yaml
```

Or you can [clean it up completely](./README.md#clean-up) and [completely
redeploy `Elafros`](./README.md#start-elafros).

## Tests

Running tests as you make changes to the code-base is pretty simple. See [the test docs](./test/README.md).

## Clean up

You can delete all of the service components with:
```shell
ko delete -f config/
```

## Telemetry

See [telemetry documentation](./docs/telemetry.md).
