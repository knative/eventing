# Broker cleanup post-0.16.0 install

The following is a log of testing commands to try out this job.

The job's log is the yaml of all resources that were deleted. The job uses
eventing's service account.

---

## kind start cluster:

```
cat <<EOF | kind create cluster --name broker --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765
- role: worker
  image: kindest/node:v1.16.9@sha256:7175872357bc85847ec4b1aba46ed1d12fa054c83ac7a8a11f5c268957fd5765
EOF
```

## install knative eventing 0.14:

```
kubectl apply --selector knative.dev/crd-install=true --filename https://github.com/knative/eventing/releases/download/v0.14.0/eventing.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.14.0/eventing.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.14.0/in-memory-channel.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.14.0/channel-broker.yaml
kubectl label namespace default knative-eventing-injection=enabled
```

## monitor

```
watch kubectl get broker,trigger,pingsource,deployments,rolebindings,serviceaccounts,roles
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
  jsonData: '{"message": "OG- 0.14 Broker"}'
  sink:
    ref:
      # Deliver events to Broker.
      apiVersion: eventing.knative.dev/v1alpha1
      kind: Broker
      name: default
---
 apiVersion: apps/v1
 kind: Deployment
 metadata:
   name: event-display
 spec:
   replicas: 1
   selector:
     matchLabels: &labels
       app: event-display
   template:
     metadata:
       labels: *labels
     spec:
       containers:
         - name: event-display
           image: gcr.io/knative-releases/knative.dev/eventing-contrib/cmd/event_display
---
   kind: Service
   apiVersion: v1
   metadata:
     name: event-display
   spec:
     selector:
       app: event-display
     ports:
     - protocol: TCP
       port: 80
       targetPort: 8080
---
apiVersion: eventing.knative.dev/v1beta1
kind: Trigger
metadata:
  annotations:
    knative-eventing-injection: enabled
  name: testevents-trigger0
  namespace: default
spec:
  broker: default
  filter:
    attributes:
      type: dev.knative.sources.ping
  subscriber:
    ref:
      apiVersion: v1
      kind: Service
      name: event-display
EOF
```

Add a broker.

```
cat <<-EOF | kubectl apply -f -
apiVersion: eventing.knative.dev/v1beta1
kind: Broker
metadata:
  name: fourteen
spec: {}
EOF
```

## Upgrade to Eventing 0.15

```
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.15.0/storage-version-migration-v0.15.0.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.15.1/eventing.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.15.0/in-memory-channel.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.15.0/channel-broker.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.15.0/mt-channel-broker.yaml
kubectl apply --filename https://github.com/knative/eventing/releases/download/v0.15.0/upgrade-to-v0.15.0.yaml
```

Add a broker.

```
cat <<-EOF | kubectl apply -f -
apiVersion: eventing.knative.dev/v1beta1
kind: Broker
metadata:
  name: fifteen
spec: {}
EOF
```

# upgrade to Eventing 0.16 (nightly)

```
kubectl apply --filename https://storage.googleapis.com/knative-nightly/eventing/latest/pre-install-to-v0.16.0.yaml
kubectl apply --filename https://storage.googleapis.com/knative-nightly/eventing/latest/eventing.yaml
kubectl apply --filename https://storage.googleapis.com/knative-nightly/eventing/latest/in-memory-channel.yaml
kubectl apply --filename https://storage.googleapis.com/knative-nightly/eventing/latest/channel-broker.yaml
kubectl apply --filename https://storage.googleapis.com/knative-nightly/eventing/latest/mt-channel-broker.yaml

rel-ko apply -f ./config/post-install/v0.16.0/
or
kubectl apply --filename https://storage.googleapis.com/knative-nightly/eventing/latest/post-install-to-v0.16.0.yaml
```

Add a broker.

```
cat <<-EOF | kubectl apply -f -
apiVersion: eventing.knative.dev/v1beta1
kind: Broker
metadata:
  name: sixteen
spec: {}
EOF
```

# clean up

```
kind delete cluster --name broker
```

## Notes:

What needs to be cleaned up:

### in knative-eventing

```
broker-controller       0/0     0            0           10m
broker-filter           0/0     0            0           10m
broker-ingress          0/0     0            0           10m
```

#### in your namespace:

```
deployment.apps/default-broker-filter    1/1     1            1           10m
deployment.apps/default-broker-ingress   1/1     1            1           10m
serviceaccount/eventing-broker-filter    1         10m
serviceaccount/eventing-broker-ingress   1         10m
rolebinding.rbac.authorization.k8s.io/eventing-broker-filter    11m
rolebinding.rbac.authorization.k8s.io/eventing-broker-ingress   11m
```
