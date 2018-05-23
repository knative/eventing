# gcp_pubsub_function

A simple function that receives Google Cloud Pub Sub events and prints out the data field after decoding
from base64 encoding. Because we do **not** have an in-cluster event delivery mechanism yet, uses an
elafros route as an endpoint.

## Prerequisites

1. [Setup your development environment](../../DEVELOPMENT.md#getting-started)
2. [Start Elafros](../../README.md#start-elafros)
3. Decide on the DNS name that git can then call. Update elafros/elafros/elaconfig.yaml domainSuffix.
For example I used aikas.org as my hostname, so my elaconfig.yaml looks like so:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ela-config
  namespace: ela-system
data:
  aikas.org: |
```

If you were already running the elafros controllers, you will need to kill the ela-controller in the ela-system namespace for it to pick up the new domain suffix.

4. Create a GCP Pub Sub topic

```shell
gcloud pubsub topics create ela-demo
```

5. Install the event sources and types for [gcp_pubsub](../gcp_pubsub/README.md)

## Running

You can deploy this to Elafros from the root directory via:
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

To make this service accessible to github, we first need to determine its ingress address
(might have to wait a little while until 'ADDRESS' gets assigned):
```shell
$ watch kubectl get ingress
NAME                              HOSTS                                                                           ADDRESS           PORTS     AGE
gcp-pubsub-function-ela-ingress   gcp-pubsub-function.default.aikas.org,*.gcp-pubsub-function.default.aikas.org   104.197.125.124   80        48s

```

Once the `ADDRESS` gets assigned to the cluster, you need to assign a DNS name for that IP address. This DNS address needs to be:
git-webhook.default.<domainsuffix you created> so for me, I would create a DNS entry from:
git-webhook.default.aikas.org pointing to 104.197.125.124
[Using GCP DNS](https://support.google.com/domains/answer/3290350)

So, you'd need to create an A record for gcp-pubsub-function.default.aikas.org pointing to 104.197.125.124

To now bind the gcp_pubsub_function for GCP PubSub messages with the function we created above, you need to
 create a Bind object. Modify sample/gcp_pubsub_function/subscribe.yaml to specify the topic and project id
 you want.

 For example, if I wanted to receive notifications to:
 project: quantum-reducer-434 topic: ela-demo, my Bind object would look like so:

```yaml
apiVersion: elafros.dev/v1alpha1
kind: Bind
metadata:
  name: gcppubsub-example
  namespace: default
spec:
  trigger:
    service: gcppubsub
    eventType: receive
    resource: quantum-reducer-434/ela-demo
    parameters:
      projectID: quantum-reducer-434
      topic: ela-demo
  action:
    routeName: gcp-pubsub-function
```

Then create the binding so that you can see changes

```shell
 kubectl create -f sample/gcp_pubsub_function/subscribe.yaml
```


This will create a subscription to the topic and create an Event Receiver that uses Pull Channel
to receive events from the topic we just created.

```shell
$gcloud pubsub subscribers list 

<snip>
ackDeadlineSeconds: 10
messageRetentionDuration: 604800s
name: projects/quantum-reducer-434/subscriptions/sub-67f12e62-95a2-4a5e-bc30-9edd29380214
pushConfig: {}
topic: projects/quantum-reducer-434/topics/ela-demo
<snip>

```

Then look at the logs for the function:

```shell
$ kubectl get pods
NAME                                                    READY     STATUS    RESTARTS   AGE
gcp-pubsub-function-00001-deployment-68864b8c7d-rgx2w   3/3       Running   0          12m


# Replace gcp-pubsub-function with the pod name from above:
$ kubectl logs gcp-pubsub-function-00001-deployment-68864b8c7d-rgx2w ela-container
```

Nothing is there, so let's change that:

```shell
$ gcloud pubsub topics publish ela-demo --message 'test message'
```

Then look at the function logs:

```shell
$ kubectl logs gcp-pubsub-function-00001-deployment-68864b8c7d-rgx2w ela-container
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
$gcloud pubsub subscribers list 
# not there anymore
```


## Cleaning up

To clean up the sample service:

```shell
ko delete -f sample/gcp_pubsub_function/
```
