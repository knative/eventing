# k8s_events_function

A simple function that receives Kubernetes events and prints them out the after decoding
from base64 encoding. This version wires the action into an existing Channel differing from
the [sample that targets a Knative route](./README.md). This is also example of where we
make use of a Receive Adapter that runs in the context of the namespace where the binding
is created. Since there's no push events, we create a deployment that attaches to k8s events
for a given namespace and then forwards them to the destination.

## Prerequisites

1. [Setup your development environment](../../DEVELOPMENT.md#getting-started)
2. [Start Knative](../../README.md#start-knative)
3. Install k8s events as an event source
4. [Install a channel and function](../hello/README.md)

```shell
ko apply -f pkg/sources/k8sevents/
```

Once deployed, you can inspect the created resources with `kubectl` commands:

```shell
# This will show the available EventSources that you can bind to:
kubectl get eventsources -oyaml

# This will show the available EventTypes that you can bind to:
kubectl get eventtypes -oyaml

```

## Creating a Service Account
Because the Receive Adapter needs to run a deployment, you need to specify what 
Service Account should be used in the target namespace for running the Receive Adapter.
Bind.Spec has a field that allows you to specify this. By default it uses "default" for
binding which typically has no priviledges, but this binding requires standing up a
deployment, so you need to either use an existing Service Account with appropriate
priviledges or create a new one. This example creates a Service Account and grants
it cluster admin access, and you probably wouldn't want to do that in production
settings, but for this example it will suffice just fine.

```shell
ko apply -f sample/k8s_events_function/serviceaccount.yaml
ko apply -f sample/k8s_events_function/serviceaccountbinding.yaml
```

## Running

Assuming you have [installed the Bus, channel and function](../hello/README.md), you
can now create a binding for k8s events that will then send k8s events by creating
a binding targeting that channel.

Note that if you are using a different Service Account than created in the example above,
you also need to specify that Service Account in the bind.spec.serviceAccountName.
Modify sample/k8s_events_function/bind-channel.yaml to specify the namespace you want to
watch events for ('default' in this example):

```yaml
apiVersion: feeds.knative.dev/v1alpha1
kind: Bind
metadata:
  name: k8s-events-example-channel
  namespace: default
spec:
  serviceAccountName: binder
  trigger:
    eventType: receiveevent
    resource: k8sevents/receiveevent
    service: k8sevents
    parameters:
      namespace: default
  action:
    channelName: aloha
```

Then create the binding so that you can see changes on that namespace

```shell
 kubectl create -f sample/k8s_events_function/bind-channel.yaml
```

This will create a receive_adapter that runs in the cluster and receives native k8s events
and pushes them to the consuming function.

```shell
$kubectl -n knative-eventing-system get pods

NAME                                                        READY     STATUS    RESTARTS   AGE
bind-controller-dddb99dfc-jzp7z                             1/1       Running   0          1d
sub-a3095905-f9c8-4f32-87ac-3c8fec9b51f9-85db55dc48-2mbm9   1/1       Running   0          1m

```

Then you can launch a deployment in the namespace that you are watching:

```shell
kubectl -n <namespace> run --image=nginx nginxtest
```

Then look at the logs for the function:

```shell
$ kubectl get pods
NAME                                                        READY     STATUS      RESTARTS   AGE
hello-00001-deployment-747d97bbdc-cxg8p                     4/4       Running     0          3d

# Replace hello-00001-deployment-XXXX with the pod name from above:
$ kubectl logs hello-00001-deployment-XXXX user-container
```

## Removing a binding

Remove the binding and things get cleaned up (including removing the receive adapter to k8s events)

```shell
kubectl delete binds k8s-events-example-channel
```

## Cleaning up

To clean up the sample service:

```shell
ko delete -f sample/k8s_events_function/
```
