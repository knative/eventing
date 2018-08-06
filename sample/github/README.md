# GitHub Flow

A GitHub webhook will be created on a repository and a Knative `Service` will be 
deployed to receive the webhook's event deliveries and forward them into a 
`Channel`, through a `Bus`, and out to the consumer via a `Subscription`. The 
`Flow` resource takes care of provisioning the webhook, the `Service`, the 
`Channel`, and the `Subscription`.

## Prerequisites

1. [Setup your development environment](../../DEVELOPMENT.md#getting-started)
2. [Start Knative](../../README.md#start-knative)
3. For GitHub to be able to call into the cluster,  
   [configure a custom domain](https://github.com/knative/docs/blob/master/serving/using-a-custom-domain.md) and 
   [assign a static IP address](https://github.com/knative/docs/blob/master/serving/gke-assigning-static-ip-address.md).
4. Install a ClusterBus

update `knative/eventing/config/buses/stub-bus.yaml`, changing `kind` to `ClusterBus`, like:

```yaml
apiVersion: channels.knative.dev/v1alpha1
kind: ClusterBus
metadata:
  name: stub
spec:
  dispatcher:
    name: dispatcher
    image: github.com/knative/eventing/pkg/buses/stub
    args: [
      "-logtostderr",
      "-stderrthreshold", "INFO",
    ]
``` 

and apply the stub bus

```shell
ko apply -f config/buses/stub
```

5. Install GitHub as an event source

```shell
ko apply -f pkg/sources/github/
```

5. Check that the GitHub is now showing up as an event source and there's an event type for *pullrequests*

```shell
kubectl get eventsources
kubectl get eventtypes
```

6. Create a [personal access token](https://github.com/settings/tokens) to GitHub repo that you can use to register
   webhooks with the GitHub API. Also decide on a token that your code will use to authenticate the incoming webhooks
   from GitHub (*accessToken*). Update `sample/github/githubsecret.yaml` with those values. If your generated access
   token is `'asdfasfdsaf'` and you choose your *secretToken* as `'password'`, you'd modify
   `sample/github/githubsecret.yaml` like so:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: githubsecret
type: Opaque
stringData:
  githubCredentials: >
    {
      "accessToken": "asdfasfdsaf",
      "secretToken": "password"
    }
```

## Running

In response to a pull request event, the _legit_ Service will add `(looks pretty legit)` to the PR title.

Deploy the _legit_ service via:

```shell
ko apply sample/github/legit-service.yaml
```

Once deployed, you can inspect the created resources with `kubectl` commands:

```shell
# This will show the Service that we created:
kubectl get service.serving.knative.dev -o yaml

# This will show the Route that the Service created:
kubectl get route -o yaml

# This will show the Configuration that the Service created:
kubectl get configurations -o yaml

# This will show the Revision that was created by the Configuration:
kubectl get revisions -o yaml
```

The `Flow` will accept the Webhook calls from GitHub and pass them to the _legit_ service.

You can inspect those resources with the following `kubectl` commands:

```shell
# This will show the available EventSources that you can generate feeds from:
kubectl get eventsources -o yaml

# This will show the available EventTypes that you can generate feeds from:
kubectl get eventtypes -o yaml
```

Configure the `Flow` to point to a GitHub repository you are able to control and update
`spec.trigger.resource` in `samples/github/flow.yaml`.  

And then apply the resources:

```shell
ko apply -f sample/github/auth.yaml
ko apply -f sample/github/flow.yaml
```   

Once deployed, you can inspect the created resources with `kubectl` commands:

```shell
# This will show the available Flows we created:
kubectl get flows -o yaml

# This will show the available Feeds created from the Flow:
kubectl get feeds -o yaml

# This will show the available Channels created from the Flow:
kubectl get channels -o yaml

# This will show the available Subscriptions created from the Flow:
kubectl get subscriptions -o yaml

# This will show the available ClusterBuses we created in the prerequisites:
kubectl get clusterbuses -o yaml
```

Then create the secret so that you can see changes:

```shell
 ko apply -f sample/github/githubsecret.yaml
```

Then create a PR for the repo you configured the webhook for, and you'll see that the Title
will be modified with the suffix '(looks pretty legit)'

## Cleaning up

To clean up the sample `Flow`:
```shell
 ko delete -f sample/github/flow.yaml
```

And you can check the repo and see the webhook has been removed

```shell
ko delete -f sample/github/
```
