# Flow Example

The flow sample includes a function that listens for k8s events on a cluster and a flow
object that wires it into the function

## Deploy

First, install the Stub bus, if not already installed. You **must** change the Kind to ClusterBus

```
ko apply -f config/buses/stub/stub-bus.yaml
```

## Install k8s events as an event source
```shell
ko apply -f pkg/sources/k8sevents/
```

## Creating a Service Account
Because the Receive Adapter needs to run a deployment, you need to specify what 
Service Account should be used in the target namespace for running the Receive Adapter.
Feed.Spec has a field that allows you to specify this. By default it uses "default" for
feed which typically has no privileges, but this feed requires standing up a
deployment, so you need to either use an existing Service Account with appropriate
priviledges or create a new one. This example creates a Service Account and grants
it cluster admin access, and you probably wouldn't want to do that in production
settings, but for this example it will suffice just fine.

```shell
ko apply -f sample/k8s_events_function/serviceaccount.yaml
ko apply -f sample/k8s_events_function/serviceaccountbinding.yaml
```

## Deploy function
Then, deploy the k8s-events function

```shell
ko apply -f sample/k8s_events_function/function.yaml
```

This will create a Route and Configuration, so make sure it's up

```
$ kubectl get pods
NAME                                                        READY     STATUS    RESTARTS   AGE
k8s-events-00001-deployment-78fb756c8b-rfwl4                3/3       Running   0          40m
```

## Create a flow wiring this into k8s events
```shell
ko apply -f sample/flow/flow.yaml
```

## Check the logs of the function
```shell
kubectl logs k8s-events-00001-deployment-78fb756c8b-rfwl4 user-container
```
