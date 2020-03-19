# Record Events Image

This stores received events along with their HTTP headers and any validation
errors, exposing a REST API on port 8392. This REST API is designed to be used
in combination with knative.dev/test/lib/EventInfoStore which allows e2e test
libraries to query the set of received events.

Also logs received events and http headers to aid debugging.
