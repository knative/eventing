# End to end tests

* [Running e2e tests](../README.md#running-e2e-tests)

## Tests

### k8s_events_test
 1. install the k8sevents as an event source (ko apply -f pkg/sources/k8sevents)
 2. create a test namespace (e2etest)
 3. install a function (ko apply -f sample/k8s_k8s_events_function/function.yaml
 4. create a binding, watching namespace #3 ((ko apply -f sample/k8s_k8s_events_function/bind.yaml)
 5. create a pod in that namespace (kubectl run --namespace rando nginx --image=nginx)
 6. check the logs of the function and check that events show up
 7. tear everything down
