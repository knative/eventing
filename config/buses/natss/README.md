# Stub - Knative Bus

Deployment steps:
1. Setup [Knative Eventing](../../../DEVELOPMENT.md)
1. Apply the 'natss' Bus `ko apply -f config/buses/natss/`
1. Create Channels that reference the 'natss' Bus

The NATSS Streaming bus uses NATS Streaming based on a simple setup, see [Natss Streaming](./100-natss.yaml) .
