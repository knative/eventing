# gitwebhook

A simple git webhook handler that demonstrates interacting with
github. 
[Modeled after GCF example](https://cloud.google.com/community/tutorials/github-auto-assign-reviewers-cloud-functions)
But by introducing a Feed object, we make the subscription to github automatically by calling a github API
and hence the developer does not have to manually wire things together in the UI, by creating the Feed object
the webhook gets created by Knative Eventing.

## Prerequisites

1. [Setup your development environment](../../DEVELOPMENT.md#getting-started)
2. [Start Knative](../../README.md#start-knative)
3. Decide on the DNS name that git can then call. Update knative/serving/config/config-domain.yaml.
For example I used aikas.org as my hostname, so my knative/serving/config/config-domain.yaml looks like so:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ela-config
  namespace: ela-system
data:
  aikas.org: |
```

If you were already running the knative controllers, you will need to update this configmap from the
root of the knative/serving

```shell
ko apply -f config/config-domain.yaml
```

4. Install Github as an event source

```shell
ko apply -f pkg/sources/github/
```

5. Check that the github is now showing up as an event source and there's an event type for pullrequests

```shell
kubectl get eventsources
kubectl get eventtypes
```

6. Create a [personal access token](https://github.com/settings/tokens) to github repo that you can use to register webhooks with the Github API. Also decide on a token that your code will authenticate the incoming webhooks from github (accessSoken). Update sample/github/githubsecret.yaml with those values. If your generated access token is 'asdfasfdsaf' and you choose your secretToken as 'password', you'd modify githubsecret.yaml like so:

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

You can deploy a function that handles the pullrequest events to Knative from the root directory via:
```shell
ko apply -f sample/github/configuration.yaml
ko apply -f sample/github/route.yaml
```

Once deployed, you can inspect the created resources with `kubectl` commands:

```shell
# This will show the Route that we created:
kubectl get route -o yaml

# This will show the Configuration that we created:
kubectl get configurations -o yaml

# This will show the Revision that was created by our configuration:
kubectl get revisions -o yaml

# This will show the available EventSources that you can generate feeds from:
kubectl get eventsources -o yaml

# This will show the available EventTypes that you can generate feeds from:
kubectl get eventtypes -o yaml

```

To make this service accessible to github, we first need to determine its ingress address
(might have to wait a little while until 'ADDRESS' gets assigned):
```shell
$ kubectl get ingress --watch
NAME                      HOSTS                           ADDRESS           PORTS     AGE
git-webhook-ingress   git-webhook.default.aikas.org   104.197.125.124   80        12m
```

Once the `ADDRESS` gets assigned to the cluster, you need to assign a DNS name for that IP address. This DNS address needs to be:
git-webhook.default.<domainsuffix you created> so for me, I would create a DNS entry from:
git-webhook.default.aikas.org pointing to 104.197.125.124
[Using GCP DNS](https://support.google.com/domains/answer/3290350)

So, you'd need to create an A record for git-webhook.default.aikas.org pointing to 104.197.125.124

To now send events via the github webhook for pull requests to the function we created above, you need to
 create a Feed object. Modify sample/github/feed.yaml to point to owner of the repo as well
 as the particular repo you want to subscribe to. So, change spec.trigger.resource with the owner/repo
 you want.

 For example, if I wanted to receive notifications to:
 github.com/inlined/robots repo, my Feed object would look like so:

```yaml
apiVersion: knative.dev/v1alpha1
kind: Feed
metadata:
  name: feed-example
  namespace: default
spec:
  trigger:
    service: github
    eventType: pullrequest
    resource: inlined/robots
    parametersFrom:
      - secretKeyRef:
          name: githubsecret
          key: githubCredentials
  action:
    routeName: git-webhook
```

Then create the secret and a feed so that you can see changes

```shell
 ko apply -f sample/github/githubsecret.yaml
 ko apply -f sample/github/feed.yaml
```

Then create a PR for the repo you configured the webhook for, and you'll see that the Title
will be modified with the suffix '(I buy it)'

## Cleaning up

To clean up the sample feed:
```shell
 ko delete -f sample/github/feed.yaml
```

And you can check the repo and see the webhook has been removed

```shell
ko delete -f sample/github/
```
