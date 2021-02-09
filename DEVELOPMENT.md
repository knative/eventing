# Development

This doc explains how to setup a development environment so you can get started
[contributing](https://www.knative.dev/contributing/) to `Knative Eventing`.
Also take a look at:

- [The pull request workflow](https://www.knative.dev/contributing/contributing/#pull-requests)
- [How to add and run tests](./test/README.md)
- [Iterating](#iterating)

## Getting started

1. [Create and checkout a repo fork](#checkout-your-fork)
1. [Install a channel implementation](#install-channels)

Once you meet these requirements, you can
[start the eventing-controller](#starting-eventing-controller).

> :information_source: If you intend to use event sinks based on Knative
> Services as described in some of our examples, consider installing
> [Knative Serving](http://github.com/knative/serving). A few
> [Knative Sandbox](https://github.com/knative-sandbox/?q=eventing&type=&language=)
> projects also have a dependency on Serving.

Before submitting a PR, see also [contribution guidelines](./CONTRIBUTING.md).

### Requirements

You must have [`ko`](https://github.com/google/ko) installed.

### Create a cluster and a repo

1. [Set up a kubernetes cluster](https://www.knative.dev/docs/install/)
   - Follow an install guide up through "Creating a Kubernetes Cluster"
   - You do _not_ need to install Istio or Knative using the instructions in the
     guide. Simply create the cluster and come back here.
   - If you _did_ install Istio/Knative following those instructions, that's
     fine too, you'll just redeploy over them, below.
1. Set up a Linux Container repository for pushing images. You can use any
   container image registry by adjusting the authentication methods and
   repository paths mentioned in the sections below.
   - [Google Container Registry quickstart](https://cloud.google.com/container-registry/docs/pushing-and-pulling)
   - [Docker Hub quickstart](https://docs.docker.com/docker-hub/)

> :information_source: You'll need to be authenticated with your
> `KO_DOCKER_REPO` before pushing images. Run `gcloud auth configure-docker` if
> you are using Google Container Registry or `docker login` if you are using
> Docker Hub.

### Setup your environment

To start your environment you'll need to set these environment variables (we
recommend adding them to your `.bashrc`):

1. `GOPATH`: If you don't have one, simply pick a directory and add
   `export GOPATH=...`
1. `$GOPATH/bin` on `PATH`: This is so that tooling installed via `go get` will
   work properly.
1. `KO_DOCKER_REPO`: The docker repository to which developer images should be
   pushed (e.g. `gcr.io/[gcloud-project]`).

> :information_source: If you are using Docker Hub to store your images, your
> `KO_DOCKER_REPO` variable should have the format `docker.io/<username>`.
> Currently, Docker Hub doesn't let you create subdirs under your username (e.g.
> `<username>/knative`).

`.bashrc` example:

```shell
export GOPATH="$HOME/go"
export PATH="${PATH}:${GOPATH}/bin"
export KO_DOCKER_REPO='gcr.io/my-gcloud-project-id'
```

### Checkout your fork

The Go tools require that you clone the repository to the
`src/knative.dev/eventing` directory in your
[`GOPATH`](https://github.com/golang/go/wiki/SettingGOPATH).

To check out this repository:

1. Create your own
   [fork of this repo](https://help.github.com/articles/fork-a-repo/)
1. Clone it to your machine:

```shell
mkdir -p ${GOPATH}/src/knative.dev
cd ${GOPATH}/src/knative.dev
git clone git@github.com:${YOUR_GITHUB_USERNAME}/eventing.git
cd eventing
git remote add upstream https://github.com/knative/eventing.git
git remote set-url --push upstream no_push
```

_Adding the `upstream` remote sets you up nicely for regularly
[syncing your fork](https://help.github.com/articles/syncing-a-fork/)._

Once you reach this point you are ready to do a full build and deploy as
follows.

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

## Install Broker

Install the
[MT Channel Broker](https://github.com/knative/eventing/tree/master/config/brokers/mt-channel-broker)
or any of the other Brokers available inside the `config/brokers/` directory.

```shell
ko apply -f config/brokers/mt-channel-broker/
```

Depending on your needs you might want to install other
[Broker implementations](https://github.com/knative/eventing/tree/master/docs/broker).

## (Optional) Install Sugar controller

If you are running full set of e2e tests, you will need to install the
[sugar controller](config/sugar/README.md).

```shell
ko apply -f config/sugar/
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

## Contributing

Please check [contribution guidelines](./CONTRIBUTING.md).

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

## Packet sniffing

While debugging an Eventing component, it could be useful to perform packet
sniffing on a container to analyze the traffic.

**Note**: this debugging method should not be used in production.

In order to do packet sniffing, you need:

- [`ko`](https://github.com/google/ko) to deploy Eventing
- [`kubectl sniff`](https://github.com/eldadru/ksniff) to deploy and collect
  `tcpdump`
- (Optional) [Wireshark](https://www.wireshark.org/) to analyze the `tcpdump`
  output

After you installed all these tools, change the base image ko uses to build
Eventing component images changing the [.ko.yaml](./.ko.yaml). You need an image
that has the `tar` tool installed, for example:

```yaml
defaultBaseImage: docker.io/debian:latest
```

Now redeploy with `ko` the component you want to sniff as explained in the above
paragraphs.

When the container is running, run:

```
kubectl sniff <POD_NAME> -n knative-eventing -o out.dump
```

Changing `<POD_NAME>` with the pod name of the component you wish to test, for
example `imc-dispatcher-85797b44c8-gllnx`. This command will dump the `tcpdump`
output with all the sniffed packets to `out.dump`. Then, you can open this file
with Wireshark using:

```
wireshark out.dump
```

If you run `kubectl sniff` without the output file name, it will open directly
Wireshark:

```
kubectl sniff <POD_NAME> -n knative-eventing
```
