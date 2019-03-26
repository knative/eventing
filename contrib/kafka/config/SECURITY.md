# Security

Kafka channel controller and dispatcher connect to brokers without
encryption and authentication by default. Encryption using SSL can be
turned on by adding certs.

## Connect over SSL(TLS)

1. Create secret from Cluster CA to verify the identity of the Kafka brokers.

   ```yaml
   kubectl create secret generic kafka-certificates  --from-file=ca.crt -n knative-eventing
   ```

   > Note: The CA certificate is to validate the Kafka broker when connecting to Kafka brokers
   > over TLS.
   >
   > If you are using Strimzi, you can create the secret from the same value in the secret
   > `<cluster>-cluster-ca-cert`.
   > e.g.
   > ```
   > kubectl get secret -n kafka my-cluster-clients-ca-cert -o json | jq -r '.data."ca.crt"' |base64 -d > ca.crt
   > kubectl create secret generic kafka-certificates  --from-file=ca.crt -n knative-eventing
   > ```


1. Define environmental variables KAFKA_CA_CERT in their Deployments.

   ```yaml
           - name: KAFKA_CA_CERT
             valueFrom:
               secretKeyRef:
                 key: ca.crt
                 name: kafka-certificates
   ```

1. Change the value of `bootstrap_servers` in configmap `kafka-channel-controller-config` for secure connection.

   ```yaml
   data:
     bootstrap_servers: kafkabroker.kafka:9093
   ```

1. Apply the change to your deployments.

   ```
   ko apply -f contrib/kafka/config/kafka.yaml
   ```
