# End to end tests

* [Running e2e tests](../README.md#running-e2e-tests)

## Tests

### k8s_events_test
 1. install the k8sevents as an event source (ko apply -f pkg/sources/k8sevents)
 2. create a test namespace (e2etest) and testfn namespace (e2etestfn)
 3. create service account and role binding (ko apply -f test/e2e/k8sevents/serviceaccount*.yaml)
 4. install a stub bus (ko apply -f test/e2e/k8sevents/stub.yaml)
 5. create a channel (ko apply -f test/e2e/k8sevents/channel.yaml)
 6. install a function (ko apply -f test/e2e/k8sevents/function.yaml)
 7. subscribe function to channel (ko apply -f test/e2e/k8sevents/subscription.yaml)
 8. create a binding, watching namespace #3 (ko apply -f test/e2e/k8sevents/bind-channel.yaml)
 9. create a pod in that namespace (ko apply -f test/e2e/k8sevents/pod.yaml)
 10. check the logs of the function and check that events show up
