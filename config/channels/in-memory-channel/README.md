# In-Memory Channels

In-memory channels are a best effort channel. They have the following
characteristics:

- **No Persistence**.
  - When a Pod goes down, messages go with it.
- **No Ordering Guarantee**.
  - There is nothing enforcing an ordering, so two messages that arrive at the
    same time may go to subscribers in any order.
  - Different downstream subscribers may see different orders.
- **No Redelivery Attempts**.
  - When a subscriber rejects a message, there is no attempts to retry sending
    it.
- **Dead Letter Sink**.
  - When a subscriber rejects a message, this message is sent to the dead letter
    sink, if present, otherwise it is dropped.

## Deployment steps:

1. Apply the `InMemoryChannel` CRD, Controller, and cluster-scoped Dispatcher.

   ```shell
   ko apply -Rf config/channels/in-memory-channel/
   ```

1. Create InMemoryChannels

   ```shell
   kubectl apply --filename - << EOF
   apiVersion: messaging.knative.dev/v1
   kind: InMemoryChannel
   metadata:
    name: foo
   EOF
   ```

## Components

The major components are:

- InMemoryChannel Controller
- InMemoryChannel Dispatcher

```shell
kubectl get deployment -n knative-eventing imc-controller
```

The InMemoryChannel Dispatcher receives and distributes all events. There is a
single Dispatcher for all in-memory Channels.

```shell
kubectl get deployment -n knative-eventing imc-dispatcher
```

### Namespace Dispatchers

By default, events will be received and dispatched by a single cluster-scoped
dispatcher components. You can also specify whether events should be received
and dispatched by the dispatcher in the same namespace as the channel definition
by adding the `eventing.knative.dev/scope: namespace` annotation. For instance:

```shell
kubectl apply --filename - << EOF
apiVersion: messaging.knative.dev/v1
kind: InMemoryChannel
metadata:
  name: foo-ns
  namespace: default
  annotations:
    eventing.knative.dev/scope: namespace
EOF
```

## Demo

InMemoryChannel should work without core eventing installed.

> Warning: `Subscriptions` are provided with Eventing Core.

1. Install a channel.

   ```shell
   kubectl apply --filename - << EOF
   apiVersion: messaging.knative.dev/v1
   kind: InMemoryChannel
   metadata:
    name: demo
   EOF
   ```

1. Install event display:

   ```shell
   ko apply -f - << EOF
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
           - name: display
             image: ko://knative.dev/eventing/cmd/event_display
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
   EOF
   ```

1. Add event-display as a subscriber of the channel.

   ```shell
   kubectl patch InMemoryChannel demo --type merge --patch '{"spec":{"subscribers":[{"subscriberUri":"http://event-display.default.svc.cluster.local","uid":"abc-123"}]}}'
   ```

1. Create a heartbeats pod:

   ```shell
   ko apply --filename - << EOF
   apiVersion: v1
   kind: Pod
   metadata:
     name: heartbeats
   spec:
     containers:
       - name: heartbeats
         image: ko://knative.dev/eventing/cmd/heartbeats
         env:
           - name: "K_SINK"
             value: "http://demo-kn-channel.default.svc.cluster.local"
           - name: POD_NAMESPACE
             valueFrom:
               fieldRef:
                 fieldPath: metadata.namespace
           - name: POD_NAME
             valueFrom:
               fieldRef:
                 fieldPath: metadata.name
   EOF
   ```

### Results

The `demo` channel should end up looking like this:

```
$ kubectl get channel demo -oyaml
apiVersion: messaging.knative.dev/v1
kind: InMemoryChannel
metadata:
    messaging.knative.dev/creator: kubernetes-admin
    messaging.knative.dev/lastModifier: kubernetes-admin
    messaging.knative.dev/subscribable: v1
  creationTimestamp: "2021-05-05T18:45:25Z"
  generation: 2
  name: demo
  namespace: default
  resourceVersion: "20083"
  uid: a3c67f17-ebe6-4906-ba8d-7d8497346629
spec:
  subscribers:
  - subscriberUri: http://event-display.default.svc.cluster.local
    uid: abc-123
status:
  address:
    url: http://demo-kn-channel.default.svc.cluster.local
  conditions:
  - lastTransitionTime: "2021-05-05T18:45:26Z"
    status: "True"
    type: Addressable
  - lastTransitionTime: "2021-05-05T18:45:26Z"
    status: "True"
    type: ChannelServiceReady
  - lastTransitionTime: "2021-05-05T18:45:26Z"
    status: "True"
    type: DispatcherReady
  - lastTransitionTime: "2021-05-05T18:45:26Z"
    status: "True"
    type: EndpointsReady
  - lastTransitionTime: "2021-05-05T18:45:26Z"
    status: "True"
    type: Ready
  - lastTransitionTime: "2021-05-05T18:45:26Z"
    status: "True"
    type: ServiceReady
  observedGeneration: 2
  subscribers:
  - ready: "True"
    uid: abc-123
```

And a `stern` output would look like this:

```shell
$ stern ".*"
heartbeats heartbeats 2021/05/05 19:26:15 sending cloudevent to http://demo-kn-channel.default.svc.cluster.local
event-display-5d859f98d7-b79d7 display ☁️  cloudevents.Event
event-display-5d859f98d7-b79d7 display Validation: valid
event-display-5d859f98d7-b79d7 display Context Attributes,
event-display-5d859f98d7-b79d7 display   specversion: 1.0
event-display-5d859f98d7-b79d7 display   type: dev.knative.eventing.samples.heartbeat
event-display-5d859f98d7-b79d7 display   source: https://knative.dev/eventing-contrib/cmd/heartbeats/#default/heartbeats
event-display-5d859f98d7-b79d7 display   id: a6a36488-028f-4fd0-9c2c-31afc389ef73
event-display-5d859f98d7-b79d7 display   time: 2021-05-05T19:26:15.452991106Z
event-display-5d859f98d7-b79d7 display   datacontenttype: application/json
event-display-5d859f98d7-b79d7 display Extensions,
event-display-5d859f98d7-b79d7 display   beats: true
event-display-5d859f98d7-b79d7 display   heart: yes
event-display-5d859f98d7-b79d7 display   the: 42
event-display-5d859f98d7-b79d7 display Data,
event-display-5d859f98d7-b79d7 display   {
event-display-5d859f98d7-b79d7 display     "id": 11,
event-display-5d859f98d7-b79d7 display     "label": ""
event-display-5d859f98d7-b79d7 display   }
```
