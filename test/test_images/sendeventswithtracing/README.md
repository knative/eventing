# Send Event with Tracing

Send test event with tracing enabled.

## Development 

Can be run without k8s for quick dev testing of code:

```
go run ./test_images/sendeventswithtracing -event-type dev.knative.test.event -event-source conformance-headers-sender-binary -event-data '{"msg":"TestSingleHeaderEvent 125e7062-bdf5-11e9-98d0-3c15c2d6cf7e"}' -event-encoding binary -sink http://localhost:8080
```

And using [eventdetails](../eventdetails) to receive.


