# NATS Streaming - Knative Bus

Deployment steps:
1. Setup [Knative Eventing](../../../DEVELOPMENT.md)
1. Apply the 'natss' bus: 
```ko apply -f config/buses/natss/```
1. Create Channels that reference the 'natss' bus

The NATSS Streaming bus uses NATS Streaming based on a simple setup, see [Natss Streaming](./100-natss.yaml) .
