# Development

This doc explains how to setup a development environment so you can get started
[contributing](https://www.knative.dev/contributing/) to `Knative Eventing`.
Also take a look at:

- [The pull request workflow](https://www.knative.dev/contributing/contributing/#pull-requests)
- [How to add and run tests](./test/README.md)
- [Iterating](#iterating)

## Getting started

1. [Create and checkout a repo fork](#checkout-your-fork)
2. [Make sure all the requirements are fullfilled](#requirements)
3. [Create a cluster and Linux Container repo](#create-a-cluster-and-a-repo)
4. [Set up the environment variables](#setup-your-environment)
5. [Start eventing controller](#starting-eventing-controller)
6. [Install the rest (Optional)](#install-channels)


> :information_source: If you intend to use event sinks based on Knative
> Services as described in some of our examples, consider installing
> [Knative Serving](http://github.com/knative/serving). A few
> [Knative Sandbox](https://github.com/knative-sandbox/?q=eventing&type=&language=)
> projects also have a dependency on Serving.

Before submitting a PR, see also [contribution guidelines](./CONTRIBUTING.md).

### Requirements

You must install these tools:

1. [`go`](https://golang.org/doc/install): The language `Knative Eventing` is
   developed with (version 1.18 or higher)
1. [`git`](https://help.github.com/articles/set-up-git/): For source control
1. [`ko`](https://github.com/google/ko): For building and deploying container
   images to Kubernetes in a single command.
1. [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/): For
   managing development environments.
1. [`bash`](https://www.gnu.org/software/bash/) v4 or higher. On macOS the
   default bash is too old, you can use [Homebrew](https://brew.sh) to install a
   later version. For running some automations, such as dependencies updates and
   code generators.

### Create a cluster and a repo

1. Set up a kubernetes cluster. You can use one of the resources below, or any other kubernetes cluster:
   - [minikube start](https://minikube.sigs.k8s.io/docs/start/)
   - [KinD quickstart](https://kind.sigs.k8s.io/docs/user/quick-start/)
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
[In-Memory-Channel](https://github.com/knative/eventing/tree/main/config/channels/in-memory-channel)
since this is the
[default channel](https://github.com/knative/docs/blob/main/docs/eventing/channels/default-channels.md).

```shell
ko apply -Rf config/channels/in-memory-channel/
```

Depending on your needs you might want to install other
[channel implementations](https://github.com/knative/docs/blob/main/docs/eventing/channels/channels-crds.md).

## Install Broker

Install the
[MT Channel Broker](https://github.com/knative/eventing/tree/main/config/brokers/mt-channel-broker)
or any of the other Brokers available inside the `config/brokers/` directory.

```shell
ko apply -f config/brokers/mt-channel-broker/
```

Depending on your needs you might want to install other
[Broker implementations](https://github.com/knative/eventing/tree/main/docs/broker).

## Install Cert-Manager

Install the Cert-manager operator to run e2e tests for TLS

```shell
kubectl apply -f third_party/cert-manager
```

Depending on your needs you might want to install other
[Broker implementations](https://github.com/knative/eventing/tree/main/docs/broker).

## Enable Sugar controller

If you are running e2e tests that leverage the Sugar Controller, you will need
to explicitly enable it.

```shell
ko apply -f test/config/sugar.yaml
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
ko apply -f config/500-controller.yaml
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

- [Accessing Metrics](https://knative.dev/docs/eventing/observability/metrics/collecting-metrics/)
- [Accessing Logs](https://knative.dev/docs/eventing/observability/logging/collecting-logs/)
- [Accessing Traces](https://www.knative.dev/docs/eventing/accessing-traces/)

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

## Debugging Knative controllers and friends locally

[Telepresence](https://www.telepresence.io/) can be leveraged to debug Knative
controllers, webhooks and similar components.

Telepresence allows you to use your local process, IDE, debugger, etc. but
Kubernetes service calls get redirected to the process on your local. Similarly
the calls on the local process goes to actual services that are running in
Kubernetes.

### Prerequisites

- Install Telepresence v2 (see the [installation instructions](https://www.getambassador.io/docs/telepresence/latest/install/) for details).
- Deploy Knative Eventing on your Kubernetes cluster.

### Connect Telepresence and intercept the controller

As a first step Telepresence needs to your Kubernetes cluster:

```
telepresence connect
```

_Hint: If this is your first time Telepresence connects to your cluster, you
need to install the traffic manager too_

```
telepresence helm install
```

As Telepresence v2 [needs a
service](https://www.getambassador.io/docs/telepresence/latest/howtos/intercepts/#intercept-a-service-in-your-own-environment)
in front of your planned intercepted component (e.g. the controller), you need
to add a Kubernetes service for your component. E.g.:

```
kubectl -n knative-eventing expose deploy/eventing-controller
```

Afterwards you can run the following command to swap the controller with the
local controller that we will start later.

```
telepresence intercept eventing-controller --namespace knative-eventing --port 8080:8080 --env-file ./eventing-controller.env
```

This will replace the `eventing-controller` deployment on the cluster with a
proxy.

It will also create a `eventing-controller.env` file which we will use later on.
The content of this `envfile` looks like this:

```
CONFIG_LOGGING_NAME=config-logging
CONFIG_OBSERVABILITY_NAME=config-observability
METRICS_DOMAIN=knative.dev/eventing
POD_NAME=eventing-controller-78b599dbb7-8kkql
SYSTEM_NAMESPACE=knative-eventing
...
```

We need to pass these environment variables later when we are starting our
controller locally.

### Debug with IntelliJ IDEA

- Install the [EnvFile
  plugin](https://plugins.jetbrains.com/plugin/7861-envfile) in IntelliJ IDEA

- Create a run configuration in IntelliJ IDEA for `cmd/controller/main.go`:

![Imgur](http://i.imgur.com/M6F3XA2.png)

- Use the `envfile`:

![Imgur](http://i.imgur.com/cdXkKNo.png)

Now, use the run configuration and start the local controller in debug mode. You
will see that the execution will pause in your breakpoints.

### Debug with VSCode

Alternatively you can use VSCode, to debug the controller.

- Create a debug configuration in VSCode. Add the following configuration to
  your `.vscode/launch.json`:

```
{
    "configurations": [
        ...
        {
            "name": "Launch Eventing Controller",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/cmd/controller/main.go",
            "envFile": "${workspaceFolder}/eventing-controller.env",
            "preLaunchTask": "intercept-eventing-controller",
            "postDebugTask": "quit-telepresence",
        }
    ]
}
```

- Debug your application as usual in VSCode

_Hint: You can also add the telepresence interception as a preLaunchTask, so you
don't have to start it every time befor you debug manually. To do so, do the
following steps_

1. Add the following tasks to your `.vscode/tasks.json`:
   ```
   {
       "version": "2.0.0",
       "tasks": [
           ...
           {
               "label": "intercept-eventing-controller",
               "type": "shell",
               "command": "telepresence quit; telepresence intercept eventing-controller --namespace knative-eventing --port 8080:8080 --env-file ${workspaceFolder}/eventing-controller.env",
           },
           {
               "label": "quit-telepresence",
               "type": "shell",
               "command": "telepresence quit"
           }
       ]
   }
   ```
2. Reference the tasks in your launch configuration (`.vscode/launch.json`):
   ```
   {
       "configurations": [
           ...
           {
               "name": "Launch Eventing Controller",
               ...
               "preLaunchTask": "intercept-eventing-controller",
               "postDebugTask": "quit-telepresence",
           }
       ]
   }
   ```

### Cleanup

To remove the proxy and revert the deployment on the cluster back to its
original state again, run:

```
telepresence quit
```

**Notes**:

- Networking works fine, but volumes (i.e. being able to access Kubernetes
  volumes from local controller) are not tested
- This method can also be used in production, but proceed with caution.
