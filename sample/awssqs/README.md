This is the instruction to deploy a message counter function using AWS SQS event source.

# Deploy AWS SQS Event Source

First, and Stub bus as ClusterBus and install the Stub bus, if not already installed:

```
ko apply -f config/buses/stub/stub-bus.yaml
```

Then deploy AWS SQS event source:

```
ko apply -f pkg/sources/awssqs
```

# Deploy Message Counter Function

First, use your AWS access ID and Key in [awssecret.yaml](./awssecret.yaml)

Then, deploy the message counter function:

```shell
ko apply -f sample/awssqs/
```

# Invoke

Once the function is installed, you send SQS messages and view the number of messages being processed (including the query request) by querying the counter pod:

```shell
export HOST_URL=$(kubectl get route aws-sqs-route  -o jsonpath='{.status.domain}')
export IP_ADDRESS=$(kubectl get node  -o 'jsonpath={.items[0].status.addresses[0].address}'):$(kubectl get svc knative-ingressgateway -n istio-system   -o 'jsonpath={.spec.ports[?(@.port==80)].nodePort}')
curl -H "Host: ${HOST_URL}" http://${IP_ADDRESS}
```

 