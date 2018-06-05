# Build

```
./build.sh
```

# Install

```
kubectl apply -f square.yaml
```

# Invoke

```
export INGRESS_HOST=$(minikube ip)
export INGRESS_PORT=$(kubectl get svc istio-ingress -n istio-system -o jsonpath='{.spec.ports[0].nodePort}')
curl -H "Host: square" -H "Content-Type: application/json" $INGRESS_HOST:$INGRESS_PORT -d "33"
```
