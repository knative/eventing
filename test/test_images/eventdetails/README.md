# Event Details Image

Validates received events and print deatails for conformance testing.

## Development 

Can be run without k8s for quick dev testing of code:

```
go run knative.dev/eventing/test/test_images/eventdetails
```

And use [sendeventswithtracing](../sendeventwithtracing) to get events
