# Security

kafka channel controller and dispatcher connect to brokers without
encryption and authentication by default. Encryption using SSL can be
turned.

## Connect over SSL(TLS)

1. Create secret from Cluster CA public key to verify the identity of the Kafka brokers.

   ```yaml
   kubectl create secret generic kafka-certificates  --from-file=ca.crt -n knative-eventing
   ```

1. Define environmental variables KAFKA_TLS_ENABLE and KAFKA_CA_CERT in their Deployments.

   ```yaml
           - name: KAFKA_TLS_ENABLE
             value: "true"
           - name: KAFKA_CA_CERT
             valueFrom:
               secretKeyRef:
                 key: ca.crt
                 name: kafka-certificates
   ```

1. Change the value of `bootstrap_servers` in configmap for secure connection.

   ```yaml
   data:
     bootstrap_servers: kafkabroker.kafka:9093
   ```

1. Apply the change to your deployments.

   ```
   ko apply -f contrib/kafka/config/kafka.yaml
   ```
