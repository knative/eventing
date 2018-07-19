The hello sample includes a Knative Service, Channel and Subscription.

# Deploy

First, install the Stub bus, if not already installed:

```
ko apply -f config/buses/stub/stub-bus.yaml
```

Then, deploy the hello function, channel and subscription:

```shell
ko apply -f sample/hello/
```

# Invoke

The hello function is reachable either directly via the route created by the hello service, or by the aloha channel.

## Function

We can use kail to watch the log output for the function. In a separate shell run:

```shell
kail -d hello-00001-deployment
```

To invoke the function directly:

```shell
# Put the Ingress Host name into an environment variable.
$ export SERVICE_HOST=$(kubectl get route hello -o jsonpath="{.status.domain}")

# Put the Ingress IP into an environment variable.
$ export SERVICE_IP=$(kubectl get svc knative-ingressgateway -n istio-system -o jsonpath="{.status.loadBalancer.ingress[*]['ip']}")

# Curl the Ingress IP "as-if" DNS were properly configured.
$ curl -H "Host: $SERVICE_HOST" -H "Content-Type: text/plain" $SERVICE_IP -d "Knative"
[response]
```

You should see a response like:

> Hello Knative, from hello-00001-deployment-5fb4b845fd-7h2lc

## Channel

Unlike with direct access, when invoking over the channel, the caller will not recieve a response directly.

We can use kail to watch the log output from the bus. In a separate shell run:

```shell
kail -d stub-bus -c dispatcher
```

To invoke the function via the channel we will run curl inside the cluster.

```shell
# Create a pod that we will use to curl from inside the cluser.
kubectl run --image=tutum/curl curl --restart=Never --command sleep inf

# curl the channel directly.
kubectl exec curl -- curl -H "Content-Type: text/plain" http://aloha-channel/ -d "Curl"

# You should now see the logs on both the channel and the function.

# Clean up the curl pod.
kubectl delete pod curl
```

This time there should be no response via curl, but you should see logging from the bus and the function indicating it received the request.
