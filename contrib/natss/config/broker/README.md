# NATS Streaming - simple installation

1. For an installation of a simple NATS Streaming server, a setup is provided:

   ```sbtshell
   kubectl create namespace natss
   kubectl label namespace natss istio-injection=enabled
   kubectl apply -n natss -f contrib/natss/config/broker/natss.yaml
   ```

   NATS Streaming is deployed as a StatefulSet, using "nats-streaming" ConfigMap
   in the namespace "natss".

For tuning NATS Streaming, see
[here](https://github.com/nats-io/nats-streaming-server#configuring)
