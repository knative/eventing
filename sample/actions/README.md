# Actions

Demo to show support for dynamic actions, disjoint from the event source's configuration.

## Prerequisites

1. [Setup your development environment](../../DEVELOPMENT.md#getting-started)
2. [Start Elafros](../../README.md#start-elafros)
3. Decide on the DNS name that git can then call. Update elafros/elafros/elaconfig.yaml domainSuffix.
For example I used ela.inlined.me as my hostname, so my elaconfig.yaml looks like so:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: ela-config
  namespace: ela-system
data:
  domainSuffix: ela.inlined.me
```

If you were already running the elafros controllers, you will need to kill the ela-controller in the ela-system namespace for it to pick up the new domain suffix.

4. Since we will be mimicing an internal service, we need access to the event-delivery service.
 Expose it with:

 ```
 kubectl expose deployment event-delivery --port=80 --target-port=9090 --type=LoadBalancer --namespace bind-system
 ```

5. Install the `sendevent` utility

```bash
go install github.com/elafros/eventing/cmd/sendevent
```

6. Save the address of your exposed `event-delivery` service.

```bash
export EVENT_DELIVERY_IP=$(kubectl get services/event-delivery --namespace bind-system -o go-template='{{ (index .status.loadBalancer.ingress 0).ip }}')
```

## Demo 1: Install Bind with a debug-logging action

### Deploying assets

You can deploy this to Elafros from the root directory via:

```shell
bazel run sample/actions:everything.apply
```

This created a handful of resources. The one most immediately useful is the `action-demo` Bind.
An event source would normally deliver events to this Bind by posting to

```
${EVENT_DELIVERY_SERVICE}/v1alpha1/namespaces/default/flows/action-demo:sendEvent
```

### Sending events

We'll do the same with the `sendevent` utility:

```bash
sendevent http://${EVENT_DELIVERY_IP}/v1alpha1/namespaces/default/flows/action-demo:sendEvent
```

(You can optionally configure the sent event by choosing any of `--event-id`, `--event-type`,
`--source`, or `--data`)

### Observing side-effects

The event processor deployed in configuration.yaml is "eventing.elafros.dev/EventLogger". This
simply emits a structured event to the logs for the "event-delivery" app. View them with:

```
kubectl logs --namespace bind-system -lapp=event-delivery
```

## Demo 2: Reroute Bind to an Elafros action.

### Deploying assets

In the previous step, you already deployed the elafros route (also named 'action-demo')
that we'll use in this step. To make the demo more exciting, edit "action.go" and change
the `databaseURL` constant to a Firebase Realtime Database URL which you are an owner of.

To make the `action-demo` Bind point to your new action, edit `bind.yaml` and change the spec.action.processor to `elafros.dev/Route` 

Apply these changes with `blaze run sample/actions:everything.apply`

### Sending events

Because we did not change anything about the (mock) event source, the same command will
now have a different effect.

```bash
sendevent http://${EVENT_DELIVERY_IP}/v1alpha1/namespaces/default/flows/action-demo:sendEvent
```

### Observing side-effects

If you load your Firebase Realtime Database, you will see new event-ids appended to the "seenEvents" node every time you run `sendevent`

## Cleaning up

To clean up the sample service:

```shell
bazel run sample/github:everything.delete
```

## Demo 3: Reroute Bind to a K8S Service

### Deploying assets

This step will reuse the same Docker image as step 2, but will host `action.go` in
a Deployment + Service. To verify that we are targeting a new backend, the Deployment
sets an environment variable to change where in the Firebase Realtime Database events
are published.

To mkae the `action-demo` bind point to the Service vrsion of our demo, edit `bind.yaml` and change the spec.action.processor to `Service`.

Apply these changes with `bazel run sample/actions:everything.apply`

### Sending events

Again, the same command will now have new effects:

```
sendevent http://${EVENT_DELIVERY_IP}/v1alpha1/namespaces/default/flows/action-demo:sendEvent
```

### Observing side-effects

If you load your Firebase Realtime Database, you will see new event-ids appended to the "seenEventsInService" node every time you run `sendevent`
