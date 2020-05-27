# Event Receiver Image

Receives events and print them as json, together with additional info

## Development

Can be run without k8s for quick dev testing of code:

```go
go run knative.dev/eventing/test/test_images/event-receiver
```

And use [sendevents](../sendevents) with to get events.
