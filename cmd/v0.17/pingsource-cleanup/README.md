# PingSource cleanup post-0.17.0 install

The following is a log of testing commands to try out this job.

The job's log is the yaml of all resources that were deleted. The job uses
eventing's service account.

---

## kind start cluster:

```
cat <<EOF | kind create cluster --name pingsource --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765
- role: worker
  image: kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765
EOF
```

## install knative eventing 0.16:

```
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.16.0/eventing-crds.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.16.0/eventing-core.yaml
```

## monitor

```
watch kubectl get pingsource,deployments,rolebindings,serviceaccounts,roles
```

## app install

```
cat <<EOF | kubectl apply -f -
apiVersion: sources.knative.dev/v1alpha2
kind: PingSource
metadata:
  name: test-ping-source
spec:
  schedule: "*/1 * * * *"
  jsonData: '{"message": "OG- 0.16 PingSource"}'
  sink:
    ref:
      # Deliver events to Deployment.
      apiVersion: v1
      kind: Service
      name: event-display
EOF
```

## Upgrade to Eventing 0.17

```
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.17.0/eventing-core.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.17.0/eventing-post-install-jobs.yaml
```

# upgrade to Eventing 0.16 (nightly)

```
kubectl apply --filename https://storage.googleapis.com/knative-nightly/eventing/latest/eventing-core.yaml
kubectl apply --filename https://storage.googleapis.com/knative-nightly/eventing/latest/eventing-post-install-jobs.yaml

or
ko apply -f ./config/post-install/v0.17.0/
```

# clean up

```
kind delete cluster --name pingsource
```

## Notes:

PingSource finalizers should be removed
