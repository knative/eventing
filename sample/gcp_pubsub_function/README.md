# gcp_pubsub_function

A simple function that receives Google Cloud Pub Sub events and prints out the data field after decoding
from base64 encoding. Because we do **not** have an in-cluster event delivery mechanism yet, uses a
Knative route as an endpoint. This is also example of where we make use of a Receive Adapter
that runs in the context of the namespace where the binding is created. Since we wanted to
demonstrate pull events, we create a deployment that attaches to the specified GCP topic and
then forwards them to the destination.

## Prerequisites

1. [Setup your development environment](../../DEVELOPMENT.md#getting-started)
2. [Start Knative](../../README.md#start-knative)
3. Decide on the DNS name that git can then call. Update knative/serving/config-domain.yaml domainSuffix.
For example I used aikas.org as my hostname, so my config-domain.yaml looks like so:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-domain
  namespace: knative-serving-system
data:
  aikas.org: |
```

If you were already running the knative controllers, you will need to re-apply the configmap.

4. Install GCP Pub Sub as an event source
```shell
ko apply -f pkg/sources/gcppubsub/
```

5. Create a GCP Pub Sub topic

```shell
gcloud pubsub topics create knative-demo
```

5. Install the event sources and types for [gcp_pubsub](../gcp_pubsub/README.md)


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
ko apply -f sample/gcp_pubsub_function/serviceaccount.yaml
ko apply -f sample/gcp_pubsub_function/serviceaccountbinding.yaml
```

## Running

You can deploy this to Knative from the root directory via:
```shell
ko apply -f sample/gcp_pubsub_function/route.yaml
ko apply -f sample/gcp_pubsub_function/configuration.yaml
```

Once deployed, you can inspect the created resources with `kubectl` commands:

```shell
# This will show the Route that we created:
kubectl get route -o yaml

# This will show the Configuration that we created:
kubectl get configurations -o yaml

# This will show the Revision that was created by our configuration:
kubectl get revisions -o yaml

# This will show the available EventSources that you can bind to:
kubectl get eventsources -oyaml

# This will show the available EventTypes that you can bind to:
kubectl get eventtypes -oyaml

```

To make this service accessible to GCP, we first need to determine its ingress address
(might have to wait a little while until 'ADDRESS' gets assigned):
```shell
$ watch kubectl get ingress
NAME                              HOSTS                                                                           ADDRESS           PORTS     AGE
gcp-pubsub-function-ingress   gcp-pubsub-function.default.aikas.org,*.gcp-pubsub-function.default.aikas.org   130.211.116.160   80        20s
```

Once the `ADDRESS` gets assigned to the cluster, you need to assign a DNS name for that IP address. This DNS address needs to be:
gcp-pubsub-function.default.<domainsuffix you created> so for me, I would create a DNS entry from:
gcp-pubsub-function.default.aikas.org pointing to 130.211.116.160
[Using GCP DNS](https://support.google.com/domains/answer/3290350)

So, you'd need to create an A record for gcp-pubsub-function.default.aikas.org pointing to 130.211.116.160

To now bind the gcp_pubsub_function for GCP PubSub messages with the function we created above, you need
to create a Bind object. Modify sample/gcp_pubsub_function/bind.yaml to specify the topic and project id
you want.
For example, if I wanted to receive notifications to:
project: quantum-reducer-434 topic: knative-demo, my Bind object would look like the one below.

You can also specify a different Service Account to use for the bind / receive watcher by changing
the spec.serviceAccountName to something else.


```yaml
apiVersion: feeds.knative.dev/v1alpha1
kind: Bind
metadata:
  name: gcppubsub-example
  namespace: default
spec:
  serviceAccountName: binder
  trigger:
    service: gcppubsub
    eventType: receive
    resource: quantum-reducer-434/knative-demo
    parameters:
      projectID: quantum-reducer-434
      topic: knative-demo
  action:
    routeName: gcp-pubsub-function
```

Then create the binding so that you can see changes

```shell
 ko apply -f sample/gcp_pubsub_function/bind.yaml
```


This will create a subscription to the topic and create an Event Receiver that uses Pull Channel
to receive events from the topic we just created.

```shell
$gcloud pubsub subscriptions list

<snip>
ackDeadlineSeconds: 10
messageRetentionDuration: 604800s
name: projects/quantum-reducer-434/subscriptions/sub-67f12e62-95a2-4a5e-bc30-9edd29380214
pushConfig: {}
topic: projects/quantum-reducer-434/topics/knative-demo
<snip>

```

Then look at the logs for the function:

```shell
$ kubectl get pods
NAME                                                    READY     STATUS    RESTARTS   AGE
gcp-pubsub-function-00001-deployment-68864b8c7d-rgx2w   3/3       Running   0          12m


# Replace gcp-pubsub-function with the pod name from above:
$ kubectl logs gcp-pubsub-function-00001-deployment-68864b8c7d-rgx2w user-container
```

Nothing is there, so let's change that:

```shell
$ gcloud pubsub topics publish knative-demo --message 'test message'
```

Then look at the function logs:

```shell
$ kubectl logs gcp-pubsub-function-00001-deployment-68864b8c7d-rgx2w user-container
2018/05/22 19:16:59 {"ID":"99171831321660","Data":"dGVzdCBtZXNzYWdl","Attributes":null,"PublishTime":"2018-05-22T19:16:59.727Z"}
2018/05/22 19:16:59 Received data: "test message"
```

## Removing a binding

Remove the binding and things get cleaned up (including removing the subscription to GCP PubSub)

```shell
kubectl delete binds gcppubsub-example
```

And now your subscription from above has been removed
```shell
$ gcloud pubsub subscriptions list
# not there anymore
```


## Cleaning up

To clean up the sample service:

```shell
ko delete -f sample/gcp_pubsub_function/
```
