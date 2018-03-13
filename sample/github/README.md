# gitwebhook

A simple git webhook handler that demonstrates interacting with
github. 
[Modeled after GCF example](https://cloud.google.com/community/tutorials/github-auto-assign-reviewers-cloud-functions)
But by introducing a Bind object, we make the subscription to github automatically by calling a github API
and hence the developer does not have to manually wire things together in the UI, by creating the Bind object
the webhook gets created by Elafros.

## Prerequisites

1. [Setup your development environment](../../DEVELOPMENT.md#getting-started)
2. [Start Elafros](../../README.md#start-elafros)
3. Decide on the DNS name that git can then call. Update sample/gitwebhook/route.yaml domainSuffix.
For example I used demostuff.aikas.org as my hostname, so my route.yaml looks like so:

```yaml
apiVersion: elafros.dev/v1alpha1
kind: Route
metadata:
  name: git-webhook
  namespace: default
spec:
  domainSuffix: demostuff.aikas.org
  traffic:
  - configuration: configuration-example
    percent: 100
```
4. Create a [personal access token[(https://github.com/settings/tokens) to github repo that you can use to bind webhooks to the Github API. Also decide on a token that your code will authenticate the incoming webhooks from github (accessSoken). Update sample/github/githubsecret.yaml with those values. If your generated access token is 'asdfasfdsaf' and you choose your secretToken as 'password', you'd modify githubsecret.yaml like so:

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

You can deploy this to Elafros from the root directory via:
```shell
bazel run sample/github:everything.create
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
NAME                                 HOSTS                     ADDRESS        PORTS     AGE
git-webhook-ela-ingress              demostuff.aikas.org       35.202.30.59   80        14s
```

Once the `ADDRESS` gets assigned to the cluster, you need to assign a DNS name for that IP address.
[Using GCP DNS](https://support.google.com/domains/answer/3290350)

So, you'd need to create an A record for demostuff.aikas.org pointing to 35.202.30.59.

To now bind the github webhook for pull requests with the function we created above, you need to
 create a Bind object. Modify sample/github/pullrequest.yaml to point to owner of the repo as well
 as the particular repo you want to subscribe to. So, change spec.parameters.[owner,repo] with the values
 you want.

 For example, if I wanted to receive notifications to:
 github.com/vaikas/misc repo, my Bind object would look like so:

```yaml
apiVersion: elafros.dev/v1alpha1
kind: Bind
metadata:
  name: bind-example
  namespace: default
spec:
 action:
    routeName: git-webhook
  source:
    eventSource: github
    eventType: pullrequest
  parameters:
    owner: vaikas
    repo: misc
  parametersFrom:
    - secretKeyRef:
        name: githubsecret
        key: githubCredentials 

```

Then create the binding so that you can see changes

```shell
 kubectl create -f sample/github/pullrequest.yaml
```


Then create a PR for the repo you configured the webhook for, and you'll see that the Title
will be modified with the suffix '(looks pretty legit to me)'

## Cleaning up

To clean up the sample service:

```shell
bazel run sample/github:everything.delete
```
