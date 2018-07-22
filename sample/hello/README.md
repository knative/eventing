# Channel Demo

> **Note**: This is an older demo, and many components have changed since this
> demo was written. In particular, the following items have broken this demo:
>
> 1.  The Subscription targets the kubernetes `Service` associated with the
>     Knative `Revision`, rather than the DNS name associated with the Knative
>     `Route`. This was an early workaround for Istio 0.6 `RouteRules`, and is
>     resolved with Istio 0.8 `VirtualServices`.
> 1.  The example provisions a stub `Bus`. It should instead provision a
>     `ClusterBus` and then create namespace-specific Channels as needed.
> 1.  `Channels` no longer provision an `Ingress`. `Channels are intended to
>     provide a within-cluster fanout mechanism, but are not intended to be
>     exposed between clusters.
> 1.  The sample payload is unstructured. It should be rewritten to send a
>     CloudEvents structured-format payload.
> 1.  The sample uses `ko` to deploy the application code. It should use Docker
>     or some other mechanism which does not require a `go install` installer.
> 1.  The sample sends events to only a single Service, which does not really
>     motivate its' usage. It should send events to two different endpoints.
> 1.  The invocation section uses `kail`, but does not reference it as a
>     requirement.

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

The service will deploy the function and expose an ingress for the function and
channel. It may take a minute to acquire an IP address, but eventually you
should see something similar to:

```
$ watch -n1 kubectl get ing
NAME            HOSTS                                                           ADDRESS    PORTS
aloha-channel   aloha                                                           1.2.3.4    80
hello-ingress   hello.default.demo-domain.com,*.hello.default.demo-domain.com   1.2.3.4    80
```

# Invoke

The hello function is reachable either directly via the route created by the
hello service, or by the aloha channel.

We can use kail to watch the log output for the function. In a separte shell
run:

```
kail -d hello-00001-deployment
```

To invoke the function directly:

```
# Put the Ingress Host name into an environment variable.
$ export SERVICE_HOST=`kubectl get route hello -o jsonpath="{.status.domain}"`

# Put the Ingress IP into an environment variable.
$ export SERVICE_IP=`kubectl get ingress hello-ingress -o jsonpath="{.status.loadBalancer.ingress[*]['ip']}"`

# Curl the Ingress IP "as-if" DNS were properly configured.
$ curl -H "Host: $SERVICE_HOST" -H "Content-Type: text/plain" $SERVICE_IP -d "Knative"
[response]
```

You should see a response like:

> Hello Knative, from hello-00001-deployment-5fb4b845fd-7h2lc

Unlike with direct access, when invoking over the channel, the caller will not
recieve a response directly.

We can use kail to watch the log output from the bus. In a separte shell run:

```
kail -d stub-bus -c dispatcher
```

To invoke the function via the channel:

```
# Put the channel hostname into an environment variable.
$ export SERVICE_HOST=`kubectl get gateway aloha-channel -o jsonpath="{.spec.servers[0].hosts[0]}"`

# Put the Istio IngressGateway IP into an environment variable.
$ export SERVICE_IP=`kubectl get svc -l istio=ingressgateway --all-namespaces -o jsonpath="{.items[0].status.loadBalancer.ingress[0].ip}"`

# Curl the Ingress IP "as-if" DNS were properly configured.
$ curl -H "Host: $SERVICE_HOST" -H "Content-Type: text/plain" $SERVICE_IP -d "Knative"
```

This time there should be no response via curl, but you should see logging from
the bus and the function indicating it received the request.
