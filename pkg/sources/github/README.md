Knative GitHub Source 
=====================

This source will enable receiving of GitHub events from within Knative Eventing.

## Events

- dev.knative.github.pullrequest

## Images

Uses two images, the feedlet and the receive adapter.

### Feedlet

A Feedlet is a container to create or destroy event source bindings.

The Feedlet is run as a Job by the Feed controller.

This Feedlet will create the Receive Adapter and register a webhook in GitHub
for the account provided.

### Receive Adapter

The Receive Adapter is a Service.serving.knative.dev resource that is registered
to receive GitHub webhook requests. The source will wrap this request in a
CloudEvent and send it to the action provided. 

## Usage

The GitHub Source expects there to be a secret in the following form:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: githubsecret
type: Opaque
stringData:
  githubCredentials: >
    {
      "accessToken": "<YOUR PERSONAL TOKEN FROM GITHUB>",
      "secretToken": "<YOUR RANDOM STRING>"
    }

```

The Feedlet requires a ServiceAccount to run as a cluster admin for the targeted namespace: 

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: feed-sa
  namespace: default
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  name: feed-admin
subjects:
  - kind: ServiceAccount
    name: feed-sa
    namespace: default
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io

```

To create a Receive Adapter (via the Feedlet), a Flow needs to exist:

```yaml
apiVersion: flows.knative.dev/v1alpha1
kind: Flow
metadata:
  name: my-github-flow
  namespace: default
spec:
  serviceAccountName: feed-sa
  trigger:
    eventType: pullrequest
    resource: <org>/<repo> # TODO: Fill this out
    service: github
    parameters:
      secretName: githubsecret
      secretKey: githubCredentials
    parametersFrom:
      - secretKeyRef:
          name: githubsecret
          key: githubCredentials
  action:
    target:
      kind: Service
      apiVersion: serving.knative.dev/v1alpha1
      name: my-service

```