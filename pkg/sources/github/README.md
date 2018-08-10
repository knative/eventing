GitHub
======

Produces the following eventtypes:

- dev.knative.github.pullrequest

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