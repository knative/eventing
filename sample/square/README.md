# Build

```
./build.sh
```

# Install

Square can be run either with or without a broker.

At this time, in order to switch between brokered and brokerless, it is recomended that you delete the previous resources and then apply the new resources.

## Brokerless

```
kubectl apply -f square.yaml -f square-brokerless.yaml
```

## Brokered

```
kubectl apply -f square.yaml -f square-brokered.yaml
```

# Invoke

```
export INGRESS_HOST=$(minikube ip)
export INGRESS_PORT=$(kubectl get svc istio-ingress -n istio-system -o jsonpath='{.spec.ports[0].nodePort}')
curl -H "Host: square" -H "Content-Type: application/json" $INGRESS_HOST:$INGRESS_PORT -d "33"
```
