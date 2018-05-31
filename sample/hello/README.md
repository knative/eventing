# Build

```
./build.sh
```

# Install

```
kubectl apply -f hello.yaml
```

# Invoke

```
export INGRESS_HOST=$(minikube ip)
export INGRESS_PORT=$(kubectl get svc istio-ingress -n istio-system -o jsonpath='{.spec.ports[0].nodePort}')
curl -X POST -H "Host: hello" $INGRESS_HOST:$INGRESS_PORT
```
