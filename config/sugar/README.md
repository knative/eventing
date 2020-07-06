# Sugar Controller

The sugar controller is an optional reconciler that watches for labels and
annotations on certain resources to inject eventing components.

To enable injection on a single Namespace:

```shell script
kubectl label namespace default eventing.knative.dev/injection=enabled
```

To enable injection based on a single Trigger in a Namespace:

```shell script
kubectl annotate trigger mytrigger eventing.knative.dev/injection=enabled
```

---

Post deployment defaulting to always inject:

```shell script
kubectl -n knative-eventing set env deployment/sugar-controller BROKER_INJECTION_DEFAULT=true
```
