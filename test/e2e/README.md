# End to end tests

- [Running e2e tests](../README.md#running-e2e-tests)

## Adding end to end tests

Knative Eventing e2e tests
[test the end to end functionality of the Knative Eventing API](#requirements)
to verify the behavior of this specific implementation.

If you want to add tests for a new `ClusterChannelProvisioner` and reuse
existing tests for it, please look into [config.go](../common/config.go) and
make corresponding changes.

If you want to add a new test case, please add one new test file under [e2e](.)
and use the `RunTests` function to run against multiple
`ClusterChannelProvisioners` that have the feature you want to test.

### Requirements

The e2e tests are used to test whether the flow of Knative Eventing is
performing as designed from start to finish.

The e2e tests **MUST**:

1. Provide frequent output describing what actions they are undertaking,
   especially before performing long running operations.
2. Follow Golang best practices.
